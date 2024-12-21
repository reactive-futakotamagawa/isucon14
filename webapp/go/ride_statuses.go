package main

import (
	"context"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/motoki317/sc"
	"github.com/oklog/ulid/v2"
)

type rideStatusManager struct {
	db             *sqlx.DB
	scacheByRideID *sc.Cache[string, []RideStatus]
}

type afterCommitFunc func(context.Context) error

func afterCommitNop(_ context.Context) error {
	return nil
}

var errorNoMatchingRideStatus = errors.New("no matching ride status")

func newRideStatusManager(ctx context.Context, db *sqlx.DB) (*rideStatusManager, error) {
	replaceByRideID := func(ctx context.Context, rideID string) ([]RideStatus, error) {
		var rideStatuses []RideStatus
		if err := db.SelectContext(ctx, &rideStatuses, "SELECT * FROM ride_statuses WHERE ride_id = ? ORDER BY created_at ASC", rideID); err != nil {
			return nil, err
		}
		return rideStatuses, nil
	}
	// FIXME: 数字はテキトー
	scacheByRideID, err := sc.New[string, []RideStatus](replaceByRideID, 1*time.Minute, 2*time.Minute)
	if err != nil {
		return nil, err
	}

	return &rideStatusManager{db, scacheByRideID}, nil
}

func (h *apiHandler) initRideStatusManager(ctx context.Context) error {
	rideStatus, err := newRideStatusManager(ctx, h.db)
	if err != nil {
		return err
	}
	h.rideStatus = rideStatus
	return nil
}

func (m *rideStatusManager) createRideStatus(ctx context.Context, tx *sqlx.Tx, rideID string, status string) (afterCommitFunc, error) {
	id := ulid.Make().String()
	_, err := tx.ExecContext(ctx, "INSERT INTO ride_statuses (id, ride_id, status) VALUES (?, ?, ?)", id, rideID, status)
	if err != nil {
		return nil, err
	}
	afterCommit := func(ctx context.Context) error {
		//m.scacheByRideID.Notify(context.Background(), rideID)
		m.scacheByRideID.Forget(rideID)
		return nil
	}
	return afterCommit, nil
}

func (h *apiHandler) createRideStatus(ctx context.Context, tx *sqlx.Tx, rideID string, status string) (afterCommitFunc, error) {
	fn, err := h.rideStatus.createRideStatus(ctx, tx, rideID, status)
	return fn, err
}

func (m *rideStatusManager) updateRideStatusAppSentAt(ctx context.Context, tx *sqlx.Tx, id string) (afterCommitFunc, error) {
	_, err := tx.ExecContext(ctx, "UPDATE ride_statuses SET app_sent_at = CURRENT_TIMESTAMP(6) WHERE id = ?", id)
	if err != nil {
		return nil, err
	}
	afterCommit := func(ctx context.Context) error {
		var rideStatus RideStatus
		if err := m.db.GetContext(ctx, &rideStatus, "SELECT * FROM ride_statuses WHERE id = ? LIMIT 1", id); err != nil {
			return err
		}
		//m.scacheByRideID.Notify(context.Background(), rideStatus.RideID)
		m.scacheByRideID.Forget(rideStatus.RideID)
		return nil
	}
	return afterCommit, err
}

func (h *apiHandler) updateRideStatusAppSentAt(ctx context.Context, tx *sqlx.Tx, id string) (afterCommitFunc, error) {
	fn, err := h.rideStatus.updateRideStatusAppSentAt(ctx, tx, id)
	return fn, err
}

func (m *rideStatusManager) updateRideStatusChairSentAt(ctx context.Context, tx *sqlx.Tx, id string) (afterCommitFunc, error) {
	_, err := tx.ExecContext(ctx, "UPDATE ride_statuses SET chair_sent_at = CURRENT_TIMESTAMP(6) WHERE id = ?", id)
	if err != nil {
		return nil, err
	}
	afterCommit := func(ctx context.Context) error {
		var rideStatus RideStatus
		if err := m.db.GetContext(ctx, &rideStatus, "SELECT * FROM ride_statuses WHERE id = ? LIMIT 1", id); err != nil {
			return err
		}
		//m.scacheByRideID.Notify(context.Background(), rideStatus.RideID)
		m.scacheByRideID.Forget(rideStatus.RideID)
		return nil
	}
	return afterCommit, err
}

func (h *apiHandler) updateRideStatusChairSentAt(ctx context.Context, tx *sqlx.Tx, rideID string) (afterCommitFunc, error) {
	fn, err := h.rideStatus.updateRideStatusChairSentAt(ctx, tx, rideID)
	return fn, err
}

// SELECT * FROM ride_statuses WHERE ride_id = ? AND app_sent_at IS NULL ORDER BY created_at ASC LIMIT 1
func (m *rideStatusManager) findRideStatusYetSentByApp(ctx context.Context, rideID string) (*RideStatus, error) {
	rideStatuses, err := m.scacheByRideID.Get(ctx, rideID)
	if err != nil {
		return nil, err
	}
	for _, rideStatus := range rideStatuses {
		if rideStatus.AppSentAt == nil {
			return &rideStatus, nil
		}
	}
	return nil, errorNoMatchingRideStatus
}

func (h *apiHandler) findRideStatusYetSentByApp(ctx context.Context, _tx *sqlx.Tx, rideID string) (*RideStatus, error) {
	rs, err := h.rideStatus.findRideStatusYetSentByApp(ctx, rideID)
	return rs, err
}

// SELECT * FROM ride_statuses WHERE ride_id = ? AND chair_sent_at IS NULL ORDER BY created_at ASC LIMIT 1
func (m *rideStatusManager) findRideStatusYetSentByChair(ctx context.Context, rideID string) (*RideStatus, error) {
	rideStatuses, err := m.scacheByRideID.Get(ctx, rideID)
	if err != nil {
		return nil, err
	}
	for _, rideStatus := range rideStatuses {
		if rideStatus.ChairSentAt == nil {
			return &rideStatus, nil
		}
	}
	return nil, errorNoMatchingRideStatus
}

func (h *apiHandler) findRideStatusYetSentByChair(ctx context.Context, tx *sqlx.Tx, rideID string) (*RideStatus, error) {
	rs, err := h.rideStatus.findRideStatusYetSentByChair(ctx, rideID)
	return rs, err
}

// SELECT status FROM ride_statuses WHERE ride_id = ? ORDER BY created_at DESC LIMIT 1
func (m *rideStatusManager) getLatestRideStatus(ctx context.Context, rideID string) (string, error) {
	rideStatuses, err := m.scacheByRideID.Get(ctx, rideID)
	if err != nil {
		return "", err
	}
	if len(rideStatuses) == 0 {
		return "", errorNoMatchingRideStatus
	}
	return rideStatuses[len(rideStatuses)-1].Status, nil
}

func (h *apiHandler) getLatestRideStatus(ctx context.Context, _tx executableGet, rideID string) (string, error) {
	if h.rideStatus == nil {
		return "", errors.New("rideStatusManager is not initialized")
	}
	status, err := h.rideStatus.getLatestRideStatus(ctx, rideID)
	return status, err
}
