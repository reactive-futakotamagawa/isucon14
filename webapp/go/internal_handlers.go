package main

import (
	"database/sql"
	"errors"
	"net/http"
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

	var chairID string
	err := h.db2.GetContext(ctx, &chairID, `
WITH near_chair_id AS (
	SELECT chair_id
	FROM (
		SELECT 
			chair_locations.*,
			ROW_NUMBER() OVER (PARTITION BY chair_id ORDER BY created_at DESC) AS ranking
		FROM chair_locations
		WHERE chair_id IN (SELECT id FROM chairs WHERE is_active = TRUE)
	) AS latest_locations
	WHERE ranking = 1
	ORDER BY  ABS(latitude - ?) + ABS(longitude - ?) ASC LIMIT 5
)
SELECT near_chair_id.chair_id
	FROM near_chair_id
LEFT JOIN rides ON rides.chair_id = near_chair_id.chair_id
LEFT JOIN ride_statuses ON ride_statuses.ride_id = rides.id
WHERE 
	rides.chair_id IN (SELECT * FROM near_chair_id)
GROUP BY
	rides.chair_id
HAVING
	COUNT(ride_statuses.chair_sent_at) % 6 = 0 
LIMIT 1
`,
		ride.PickupLatitude, ride.PickupLongitude)
	if errors.Is(err, sql.ErrNoRows) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	// matched := &Chair{}
	// empty := false
	// for i := 0; i < 10; i++ { // N+1
	// 	if err := h.db2.GetContext(ctx, matched, "SELECT * FROM chairs INNER JOIN (SELECT id FROM chairs WHERE is_active = TRUE ORDER BY RAND() LIMIT 1) AS tmp ON chairs.id = tmp.id LIMIT 1"); err != nil {
	// 		if errors.Is(err, sql.ErrNoRows) {
	// 			w.WriteHeader(http.StatusNoContent)
	// 			return
	// 		}
	// 		writeError(w, http.StatusInternalServerError, err)
	// 	}

	// 	if err := h.db.GetContext(ctx, &empty, `SELECT
	// 		COUNT(*) = 0
	// 	FROM (
	// 		SELECT
	// 			COUNT(chair_sent_at) = 6 AS completed
	// 		FROM ride_statuses
	// 			WHERE ride_id IN (SELECT id FROM rides WHERE chair_id = ?)
	// 		GROUP BY ride_id
	// 	) is_completed
	// 		WHERE completed = FALSE`,
	// 		matched.ID); err != nil {
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

	if _, err := h.db.ExecContext(ctx, "UPDATE rides SET chair_id = ? WHERE id = ?", chairID, ride.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
