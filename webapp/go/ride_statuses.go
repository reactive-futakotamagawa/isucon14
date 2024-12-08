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
