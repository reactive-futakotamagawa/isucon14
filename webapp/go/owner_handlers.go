package main

import (
	"cmp"
	"database/sql"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/oklog/ulid/v2"
)

const (
	initialFare     = 500
	farePerDistance = 100
)

type ownerPostOwnersRequest struct {
	Name string `json:"name"`
}

type ownerPostOwnersResponse struct {
	ID                 string `json:"id"`
	ChairRegisterToken string `json:"chair_register_token"`
}

func (h *apiHandler) ownerPostOwners(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req := &ownerPostOwnersRequest{}
	if err := bindJSON(r, req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, errors.New("some of required fields(name) are empty"))
		return
	}

	ownerID := ulid.Make().String()
	accessToken := secureRandomStr(32)
	chairRegisterToken := secureRandomStr(32)

	_, err := h.db2.ExecContext(
		ctx,
		"INSERT INTO owners (id, name, access_token, chair_register_token) VALUES (?, ?, ?, ?)",
		ownerID, req.Name, accessToken, chairRegisterToken,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Path:  "/",
		Name:  "owner_session",
		Value: accessToken,
	})

	writeJSON(w, http.StatusCreated, &ownerPostOwnersResponse{
		ID:                 ownerID,
		ChairRegisterToken: chairRegisterToken,
	})
}

type chairSales struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Sales int    `json:"sales"`
}

type modelSales struct {
	Model string `json:"model"`
	Sales int    `json:"sales"`
}

type ownerGetSalesResponse struct {
	TotalSales int          `json:"total_sales"`
	Chairs     []chairSales `json:"chairs"`
	Models     []modelSales `json:"models"`
}

func (h *apiHandler) ownerGetSales(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	since := time.Unix(0, 0)
	until := time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)
	if r.URL.Query().Get("since") != "" {
		parsed, err := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		since = time.UnixMilli(parsed)
	}
	if r.URL.Query().Get("until") != "" {
		parsed, err := strconv.ParseInt(r.URL.Query().Get("until"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		until = time.UnixMilli(parsed)
	}

	owner := r.Context().Value("owner").(*Owner)

	tx, err := BeginMultiTx(h.db, h.db2)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer tx.Rollback()

	chairs := []Chair{}
	if err := tx.tx1.SelectContext(ctx, &chairs, "SELECT * FROM chairs WHERE owner_id = ?", owner.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	res := ownerGetSalesResponse{
		TotalSales: 0,
	}

	modelSalesByModel := map[string]int{}
	for _, chair := range chairs {
		rides := []Ride{}
		if err := tx.tx1.SelectContext(ctx, &rides, "SELECT rides.* FROM rides JOIN ride_statuses ON rides.id = ride_statuses.ride_id WHERE chair_id = ? AND status = 'COMPLETED' AND updated_at BETWEEN ? AND ? + INTERVAL 999 MICROSECOND", chair.ID, since, until); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}

		sales := sumSales(rides)
		res.TotalSales += sales

		res.Chairs = append(res.Chairs, chairSales{
			ID:    chair.ID,
			Name:  chair.Name,
			Sales: sales,
		})

		modelSalesByModel[chair.Model] += sales
	}

	models := []modelSales{}
	for model, sales := range modelSalesByModel {
		models = append(models, modelSales{
			Model: model,
			Sales: sales,
		})
	}
	res.Models = models

	writeJSON(w, http.StatusOK, res)
}

func sumSales(rides []Ride) int {
	sale := 0
	for _, ride := range rides {
		sale += calculateSale(ride)
	}
	return sale
}

func calculateSale(ride Ride) int {
	return calculateFare(ride.PickupLatitude, ride.PickupLongitude, ride.DestinationLatitude, ride.DestinationLongitude)
}

type chairWithDetail struct {
	ID                     string       `db:"id"`
	OwnerID                string       `db:"owner_id"`
	Name                   string       `db:"name"`
	AccessToken            string       `db:"access_token"`
	Model                  string       `db:"model"`
	IsActive               bool         `db:"is_active"`
	CreatedAt              time.Time    `db:"created_at"`
	UpdatedAt              time.Time    `db:"updated_at"`
	TotalDistance          int          `db:"total_distance"`
	TotalDistanceUpdatedAt sql.NullTime `db:"total_distance_updated_at"`
}

type ownerGetChairResponse struct {
	Chairs []ownerGetChairResponseChair `json:"chairs"`
}

type ownerGetChairResponseChair struct {
	ID                     string `json:"id"`
	Name                   string `json:"name"`
	Model                  string `json:"model"`
	Active                 bool   `json:"active"`
	RegisteredAt           int64  `json:"registered_at"`
	TotalDistance          int    `json:"total_distance"`
	TotalDistanceUpdatedAt *int64 `json:"total_distance_updated_at,omitempty"`
}

func (h *apiHandler) ownerGetChairs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	owner := ctx.Value("owner").(*Owner)

	chairs := []Chair{}
	if err := h.db.SelectContext(ctx, &chairs, "SELECT * FROM chairs WHERE owner_id = ?", owner.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	chairIDs := make([]string, 0, len(chairs))
	for _, chair := range chairs {
		chairIDs = append(chairIDs, chair.ID)
	}

	chairMap := make(map[string]Chair, len(chairs))
	for _, chair := range chairs {
		chairMap[chair.ID] = chair
	}

	query, args, err := sqlx.In("SELECT * FROM chair_locations WHERE chair_id IN (?) ORDER BY created_at ASC", chairIDs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	chairLocations := []ChairLocation{}
	if err := h.db.SelectContext(ctx, &chairLocations, query, args...); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	chairLocationsMap := make(map[string][]ChairLocation, len(chairLocations))
	for _, chair := range chairs {
		chairLocationsMap[chair.ID] = []ChairLocation{}
	}
	for _, chairLocation := range chairLocations {
		if _, ok := chairLocationsMap[chairLocation.ChairID]; !ok {
			chairLocationsMap[chairLocation.ChairID] = []ChairLocation{}
		}
		chairLocationsMap[chairLocation.ChairID] = append(chairLocationsMap[chairLocation.ChairID], chairLocation)
	}

	chairDetails := make([]chairWithDetail, 0, len(chairLocationsMap))
	for chairID, locations := range chairLocationsMap {

		distance := 0
		for i := 1; i < len(locations); i++ {
			distance += abs(locations[i].Latitude-locations[i-1].Latitude) + abs(locations[i].Longitude-locations[i-1].Longitude)
		}
		var updatedAt sql.NullTime
		if len(locations) != 0 {
			updatedAt = sql.NullTime{
				Time:  locations[len(locations)-1].CreatedAt,
				Valid: true,
			}
		}

		chairDetails = append(chairDetails, chairWithDetail{
			ID:                     chairID,
			OwnerID:                chairMap[chairID].OwnerID,
			Name:                   chairMap[chairID].Name,
			AccessToken:            chairMap[chairID].AccessToken,
			Model:                  chairMap[chairID].Model,
			IsActive:               chairMap[chairID].IsActive,
			CreatedAt:              chairMap[chairID].CreatedAt,
			UpdatedAt:              chairMap[chairID].UpdatedAt,
			TotalDistance:          distance,
			TotalDistanceUpdatedAt: updatedAt,
		})
	}

	// 	chairLocations := []chairWithDetail{}
	// 	if err := h.db.SelectContext(ctx, &chairLocations, `SELECT id,
	//        owner_id,
	//        name,
	//        access_token,
	//        model,
	//        is_active,
	//        created_at,
	//        updated_at,
	//        IFNULL(total_distance, 0) AS total_distance,
	//        total_distance_updated_at
	// FROM chairs
	//        LEFT JOIN (SELECT chair_id,
	//                           SUM(IFNULL(distance, 0)) AS total_distance,
	//                           MAX(created_at)          AS total_distance_updated_at
	//                    FROM (SELECT chair_id,
	//                                 created_at,
	//                                 ABS(latitude - LAG(latitude) OVER (PARTITION BY chair_id ORDER BY created_at)) +
	//                                 ABS(longitude - LAG(longitude) OVER (PARTITION BY chair_id ORDER BY created_at)) AS distance
	//                          FROM chair_locations) tmp
	//                    GROUP BY chair_id) distance_table ON distance_table.chair_id = chairs.id
	// WHERE owner_id = ?
	// `, owner.ID); err != nil {
	// 		writeError(w, http.StatusInternalServerError, err)
	// 		return
	// 	}

	slices.SortStableFunc(chairDetails, func(i, j chairWithDetail) int {
		return cmp.Compare(i.ID, j.ID)
	})

	res := ownerGetChairResponse{}
	for _, chair := range chairDetails {
		c := ownerGetChairResponseChair{
			ID:            chair.ID,
			Name:          chair.Name,
			Model:         chair.Model,
			Active:        chair.IsActive,
			RegisteredAt:  chair.CreatedAt.UnixMilli(),
			TotalDistance: chair.TotalDistance,
		}
		if chair.TotalDistanceUpdatedAt.Valid {
			t := chair.TotalDistanceUpdatedAt.Time.UnixMilli()
			c.TotalDistanceUpdatedAt = &t
		}
		res.Chairs = append(res.Chairs, c)
	}
	writeJSON(w, http.StatusOK, res)
}
