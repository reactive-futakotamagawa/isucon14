package main

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/jmoiron/sqlx"
)

// このAPIをインスタンス内から一定間隔で叩かせることで、椅子とライドをマッチングさせる
func (h *apiHandler) internalGetMatching(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// MEMO: 一旦最も待たせているリクエストに適当な空いている椅子マッチさせる実装とする。おそらくもっといい方法があるはず…
	ride := &Ride{}
	if err := h.db.GetContext(ctx, ride, `SELECT * FROM rides WHERE chair_id IS NULL ORDER BY created_at LIMIT 1`); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	var activeChairs []Chair
	if err := h.db.SelectContext(ctx, &activeChairs, "SELECT * FROM chairs WHERE is_active = TRUE"); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	var notCompletedRideIDs []string
	if err := h.db.SelectContext(ctx, &notCompletedRideIDs, "SELECT r.id as cnt FROM rides r JOIN ride_statuses rs ON r.id = rs.ride_id GROUP BY (r.id) HAVING cnt < 6"); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if len(notCompletedRideIDs) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	var notCompletedRides []Ride
	query, args, err := sqlx.In("SELECT * FROM rides WHERE id IN (?)", notCompletedRideIDs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := h.db.SelectContext(ctx, &notCompletedRides, query, args...); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	usedChairIDs := make(map[string]struct{}, len(notCompletedRides))
	for _, ride := range notCompletedRides {
		if ride.ChairID.Valid {
			usedChairIDs[ride.ChairID.String] = struct{}{}
		}
	}

	okChairIDs := make([]string, 0, len(activeChairs))
	for _, chair := range activeChairs {
		if _, ok := usedChairIDs[chair.ID]; !ok {
			okChairIDs = append(okChairIDs, chair.ID)
		}
	}

	if len(okChairIDs) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	var matchedChair struct {
		ID       string `db:"id"`
		Distance int    `db:"distance"`
	}
	q, arg, err := sqlx.In("SELECT id, (ABS(latitude - ?) + ABS(longitude - ?)) AS distance FROM current_chair_locations WHERE id IN (?) ORDER BY distance ASC LIMIT 1", ride.PickupLatitude, ride.PickupLongitude, okChairIDs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := h.db.GetContext(ctx, &matchedChair, q, arg...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	// matched := &Chair{}
	// empty := false
	// for i := 0; i < 10; i++ {
	// 	if err := h.db.GetContext(ctx, matched, "SELECT * FROM chairs INNER JOIN (SELECT id FROM chairs WHERE is_active = TRUE ORDER BY RAND() LIMIT 1) AS tmp ON chairs.id = tmp.id LIMIT 1"); err != nil {
	// 		if errors.Is(err, sql.ErrNoRows) {
	// 			w.WriteHeader(http.StatusNoContent)
	// 			return
	// 		}
	// 		writeError(w, http.StatusInternalServerError, err)
	// 	}

	// 	if err := h.db.GetContext(ctx, &empty, "SELECT COUNT(*) = 0 FROM (SELECT COUNT(chair_sent_at) = 6 AS completed FROM ride_statuses WHERE ride_id IN (SELECT id FROM rides WHERE chair_id = ?) GROUP BY ride_id) is_completed WHERE completed = FALSE", matched.ID); err != nil {
	// 		writeError(w, http.StatusInternalServerError, err)
	// 		return
	// 	}
	// 	if empty {
	// 		break
	// 	}
	// }
	// if !empty {
	// 	w.WriteHeader(http.StatusNoContent)
	// 	return
	// }

	if _, err := h.db.ExecContext(ctx, "UPDATE rides SET chair_id = ? WHERE id = ?", matchedChair.ID, ride.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
