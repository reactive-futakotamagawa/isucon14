package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/motoki317/sc"
)

func (h *apiHandler) appAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		c, err := r.Cookie("app_session")
		if errors.Is(err, http.ErrNoCookie) || c.Value == "" {
			writeError(w, http.StatusUnauthorized, errors.New("app_session cookie is required"))
			return
		}
		accessToken := c.Value
		user := &User{}
		err = h.db2.GetContext(ctx, user, "SELECT * FROM users WHERE access_token = ?", accessToken)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusUnauthorized, errors.New("invalid access token"))
				return
			}
			writeError(w, http.StatusInternalServerError, err)
			return
		}

		ctx = context.WithValue(ctx, "user", user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *apiHandler) ownerAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		c, err := r.Cookie("owner_session")
		if errors.Is(err, http.ErrNoCookie) || c.Value == "" {
			writeError(w, http.StatusUnauthorized, errors.New("owner_session cookie is required"))
			return
		}
		accessToken := c.Value
		owner := &Owner{}
		if err := h.db2.GetContext(ctx, owner, "SELECT * FROM owners WHERE access_token = ?", accessToken); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusUnauthorized, errors.New("invalid access token"))
				return
			}
			writeError(w, http.StatusInternalServerError, err)
			return
		}

		ctx = context.WithValue(ctx, "owner", owner)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type cacheCtxDBKey struct{}

var cacheCtxDBKeyVal = cacheCtxDBKey{}

var chairAccessTokenCache = sc.NewMust(func(ctx context.Context, token string) (*Chair, error) {
	db := ctx.Value(cacheCtxDBKeyVal).(*sqlx.DB)

	chair := &Chair{}
	if err := db.GetContext(ctx, chair, `SELECT * FROM chairs WHERE access_token = ?`, token); err != nil {
		return nil, err
	}
	return chair, nil
}, time.Minute, time.Minute)

func (h *apiHandler) chairAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		c, err := r.Cookie("chair_session")
		if errors.Is(err, http.ErrNoCookie) || c.Value == "" {
			writeError(w, http.StatusUnauthorized, errors.New("chair_session cookie is required"))
			return
		}
		accessToken := c.Value
		// chair := &Chair{}
		ctx = context.WithValue(ctx, cacheCtxDBKeyVal, h.db2) // ここでdb渡す！
		chair, err := chairAccessTokenCache.Get(ctx, accessToken)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusUnauthorized, errors.New("invalid access token"))
				return
			}
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		// err = h.db.GetContext(ctx, chair, "SELECT * FROM chairs WHERE access_token = ?", accessToken)
		// if err != nil {
		// 	if errors.Is(err, sql.ErrNoRows) {
		// 		writeError(w, http.StatusUnauthorized, errors.New("invalid access token"))
		// 		return
		// 	}
		// 	writeError(w, http.StatusInternalServerError, err)
		// 	return
		// }

		ctx = context.WithValue(ctx, "chair", chair)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
