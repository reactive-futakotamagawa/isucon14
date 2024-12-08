package main

import (
	"database/sql"
	"time"
)

type Chair struct {
	ID          string    `h.db:"id"`
	OwnerID     string    `h.db:"owner_id"`
	Name        string    `h.db:"name"`
	Model       string    `h.db:"model"`
	IsActive    bool      `h.db:"is_active"`
	AccessToken string    `h.db:"access_token"`
	CreatedAt   time.Time `h.db:"created_at"`
	UpdatedAt   time.Time `h.db:"updated_at"`
}

type ChairModel struct {
	Name  string `h.db:"name"`
	Speed int    `h.db:"speed"`
}

type ChairLocation struct {
	ID        string    `h.db:"id"`
	ChairID   string    `h.db:"chair_id"`
	Latitude  int       `h.db:"latitude"`
	Longitude int       `h.db:"longitude"`
	CreatedAt time.Time `h.db:"created_at"`
}

type User struct {
	ID             string    `h.db:"id"`
	Username       string    `h.db:"username"`
	Firstname      string    `h.db:"firstname"`
	Lastname       string    `h.db:"lastname"`
	DateOfBirth    string    `h.db:"date_of_birth"`
	AccessToken    string    `h.db:"access_token"`
	InvitationCode string    `h.db:"invitation_code"`
	CreatedAt      time.Time `h.db:"created_at"`
	UpdatedAt      time.Time `h.db:"updated_at"`
}

type PaymentToken struct {
	UserID    string    `h.db:"user_id"`
	Token     string    `h.db:"token"`
	CreatedAt time.Time `h.db:"created_at"`
}

type Ride struct {
	ID                   string         `h.db:"id"`
	UserID               string         `h.db:"user_id"`
	ChairID              sql.NullString `h.db:"chair_id"`
	PickupLatitude       int            `h.db:"pickup_latitude"`
	PickupLongitude      int            `h.db:"pickup_longitude"`
	DestinationLatitude  int            `h.db:"destination_latitude"`
	DestinationLongitude int            `h.db:"destination_longitude"`
	Evaluation           *int           `h.db:"evaluation"`
	CreatedAt            time.Time      `h.db:"created_at"`
	UpdatedAt            time.Time      `h.db:"updated_at"`
}

type RideStatus struct {
	ID          string     `h.db:"id"`
	RideID      string     `h.db:"ride_id"`
	Status      string     `h.db:"status"`
	CreatedAt   time.Time  `h.db:"created_at"`
	AppSentAt   *time.Time `h.db:"app_sent_at"`
	ChairSentAt *time.Time `h.db:"chair_sent_at"`
}

type Owner struct {
	ID                 string    `h.db:"id"`
	Name               string    `h.db:"name"`
	AccessToken        string    `h.db:"access_token"`
	ChairRegisterToken string    `h.db:"chair_register_token"`
	CreatedAt          time.Time `h.db:"created_at"`
	UpdatedAt          time.Time `h.db:"updated_at"`
}

type Coupon struct {
	UserID    string    `h.db:"user_id"`
	Code      string    `h.db:"code"`
	Discount  int       `h.db:"discount"`
	CreatedAt time.Time `h.db:"created_at"`
	UsedBy    *string   `h.db:"used_by"`
}
