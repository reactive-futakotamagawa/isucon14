package main

import (
	"context"

	"github.com/jmoiron/sqlx"
	"github.com/oklog/ulid/v2"
)

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
