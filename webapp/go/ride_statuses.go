package main

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/motoki317/sc"
	"github.com/oklog/ulid/v2"
)

type rideStatusManager struct {
	scache *sc.Cache[string, []RideStatus]
}

func newRideStatusManager(db *sqlx.DB) (*rideStatusManager, error) {
	replace := func(ctx context.Context, rideID string) ([]RideStatus, error) {
		var rideStatuses []RideStatus
		if err := db.SelectContext(ctx, &rideStatuses, "SELECT * FROM ride_statuses WHERE ride_id = ? ORDER BY created_at ASC", rideID); err != nil {
			return nil, err
		}
		return rideStatuses, nil
	}
	// FIXME: 数字はテキトー
	scache, err := sc.New[string, []RideStatus](replace, 1*time.Second, 2*time.Second, sc.WithLRUBackend(1000))
	if err != nil {
		return nil, err
	}
	return &rideStatusManager{scache: scache}, nil
}

func (h *apiHandler) initRideStatusManager() error {
	rideStatus, err := newRideStatusManager(h.db)
	if err != nil {
		return err
	}
	h.rideStatus = rideStatus
	return nil
}

func (h *apiHandler) createRideStatus(ctx context.Context, tx *sqlx.Tx, rideID string, status string) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO ride_statuses (id, ride_id, status) VALUES (?, ?, ?)", ulid.Make().String(), rideID, status)
	return err
}

func (h *apiHandler) updateRideStatusAppSentAt(ctx context.Context, tx *sqlx.Tx, rideID string) error {
	_, err := tx.ExecContext(ctx, "UPDATE ride_statuses SET app_sent_at = CURRENT_TIMESTAMP(6) WHERE id = ?", rideID)
	return err
}

func (h *apiHandler) updateRideStatusChairSentAt(ctx context.Context, tx *sqlx.Tx, rideID string) error {
	_, err := tx.ExecContext(ctx, "UPDATE ride_statuses SET chair_sent_at = CURRENT_TIMESTAMP(6) WHERE id = ?", rideID)
	return err
}

func (h *apiHandler) findRideStatusYetSentByApp(ctx context.Context, tx *sqlx.Tx, rideID string) (*RideStatus, error) {
	var yetSentRideStatus RideStatus
	if err := tx.GetContext(ctx, &yetSentRideStatus, "SELECT * FROM ride_statuses WHERE ride_id = ? AND app_sent_at IS NULL ORDER BY created_at ASC LIMIT 1", rideID); err != nil {
		return nil, err
	}
	return &yetSentRideStatus, nil
}

func (h *apiHandler) findRideStatusYetSentByChair(ctx context.Context, tx *sqlx.Tx, rideID string) (*RideStatus, error) {
	var yetSentRideStatus RideStatus
	if err := tx.GetContext(ctx, &yetSentRideStatus, "SELECT * FROM ride_statuses WHERE ride_id = ? AND chair_sent_at IS NULL ORDER BY created_at ASC LIMIT 1", rideID); err != nil {
		return nil, err
	}
	return &yetSentRideStatus, nil
}

func (h *apiHandler) getLatestRideStatus(ctx context.Context, tx executableGet, rideID string) (string, error) {
	status := ""
	if err := tx.GetContext(ctx, &status, `SELECT status FROM ride_statuses WHERE ride_id = ? ORDER BY created_at DESC LIMIT 1`, rideID); err != nil {
		return "", err
	}
	return status, nil
}
