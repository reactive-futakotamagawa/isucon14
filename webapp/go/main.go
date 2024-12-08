package main

import (
	"bytes"
	"context"
	crand "crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/kaz/pprotein/integration/standalone"
)

// var db *sqlx.DB

func main() {
	go standalone.Integrate(":8888")
	mux := setup()
	slog.Info("Listening on :8080")

	if os.Getenv("USE_SOCKET") == "1" {
		fmt.Println("USE_SOCKET")

		// ここからソケット接続設定 ---
		socket_file := "/tmp/app.sock"
		os.Remove(socket_file)

		l, err := net.Listen("unix", socket_file)
		if err != nil {
			log.Fatal(err)
		}

		// go runユーザとnginxのユーザ（グループ）を同じにすれば777じゃなくてok
		err = os.Chmod(socket_file, 0777)
		if err != nil {
			log.Fatal(err)
		}

		http.Serve(l, mux)
		// ここまで ---
	} else {
		http.ListenAndServe(":8080", mux)
	}
}

func setup() http.Handler {
	host := os.Getenv("ISUCON_DB_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	port := os.Getenv("ISUCON_DB_PORT")
	if port == "" {
		port = "3306"
	}
	_, err := strconv.Atoi(port)
	if err != nil {
		panic(fmt.Sprintf("failed to convert DB port number from ISUCON_DB_PORT environment variable into int: %v", err))
	}
	user := os.Getenv("ISUCON_DB_USER")
	if user == "" {
		user = "isucon"
	}
	password := os.Getenv("ISUCON_DB_PASSWORD")
	if password == "" {
		password = "isucon"
	}
	dbname := os.Getenv("ISUCON_DB_NAME")
	if dbname == "" {
		dbname = "isuride"
	}

	dbConfig := mysql.NewConfig()
	dbConfig.User = user
	dbConfig.Passwd = password
	dbConfig.Addr = net.JoinHostPort(host, port)
	dbConfig.Net = "tcp"
	dbConfig.DBName = dbname
	dbConfig.ParseTime = true
	dbConfig.InterpolateParams = true

	db, err := sqlx.Connect("mysql", dbConfig.FormatDSN())
	if err != nil {
		panic(err)
	}

	host2 := os.Getenv("ISUCON_DB_HOST2")
	if host == "" {
		host = "127.0.0.1"
	}
	port2 := os.Getenv("ISUCON_DB_PORT")
	if port == "" {
		port = "3306"
	}
	_, err = strconv.Atoi(port)
	if err != nil {
		panic(fmt.Sprintf("failed to convert DB port number from ISUCON_DB_PORT environment variable into int: %v", err))
	}
	user2 := os.Getenv("ISUCON_DB_USER")
	if user == "" {
		user = "isucon"
	}
	password2 := os.Getenv("ISUCON_DB_PASSWORD")
	if password == "" {
		password = "isucon"
	}
	dbname2 := os.Getenv("ISUCON_DB_NAME")
	if dbname == "" {
		dbname = "isuride"
	}

	dbConfig2 := mysql.NewConfig()
	dbConfig2.User = user2
	dbConfig2.Passwd = password2
	dbConfig2.Addr = net.JoinHostPort(host2, port2)
	dbConfig2.Net = "tcp"
	dbConfig2.DBName = dbname2
	dbConfig2.ParseTime = true

	db2, err := sqlx.Connect("mysql", dbConfig2.FormatDSN())
	if err != nil {
		panic(err)
	}

	h := newHandler(db, db2)
	mux := chi.NewRouter()
	mux.Use(middleware.Logger)
	mux.Use(middleware.Recoverer)

	dbOnly := os.Getenv("DB_ONLY")
	if dbOnly == "1" {
		mux.HandleFunc("POST /api/db/initialize", h.dbInitialize)
		return mux
	}
	mux.HandleFunc("POST /api/initialize", h.postInitialize)

	// app handlers
	{
		mux.HandleFunc("POST /api/app/users", h.appPostUsers)

		authedMux := mux.With(h.appAuthMiddleware)
		authedMux.HandleFunc("POST /api/app/payment-methods", h.appPostPaymentMethods)
		authedMux.HandleFunc("GET /api/app/rides", h.appGetRides)
		authedMux.HandleFunc("POST /api/app/rides", h.appPostRides)
		authedMux.HandleFunc("POST /api/app/rides/estimated-fare", h.appPostRidesEstimatedFare)
		authedMux.HandleFunc("POST /api/app/rides/{ride_id}/evaluation", h.appPostRideEvaluatation)
		authedMux.HandleFunc("GET /api/app/notification", h.appGetNotification)
		authedMux.HandleFunc("GET /api/app/nearby-chairs", h.appGetNearbyChairs)
	}

	// owner handlers
	{
		mux.HandleFunc("POST /api/owner/owners", h.ownerPostOwners)

		authedMux := mux.With(h.ownerAuthMiddleware)
		authedMux.HandleFunc("GET /api/owner/sales", h.ownerGetSales)
		authedMux.HandleFunc("GET /api/owner/chairs", h.ownerGetChairs)
	}

	// chair handlers
	{
		mux.HandleFunc("POST /api/chair/chairs", h.chairPostChairs)

		authedMux := mux.With(h.chairAuthMiddleware)
		authedMux.HandleFunc("POST /api/chair/activity", h.chairPostActivity)
		authedMux.HandleFunc("POST /api/chair/coordinate", h.chairPostCoordinate)
		authedMux.HandleFunc("GET /api/chair/notification", h.chairGetNotification)
		authedMux.HandleFunc("POST /api/chair/rides/{ride_id}/status", h.chairPostRideStatus)
	}

	// internal handlers
	{
		mux.HandleFunc("GET /api/internal/matching", h.internalGetMatching)
	}

	return mux
}

type MultiDBTx struct {
	tx1 *sqlx.Tx // DB1のトランザクション
	tx2 *sqlx.Tx // DB2のトランザクション
}

// BeginMultiTx は複数のデータベーストランザクションを開始します
func BeginMultiTx(db1, db2 *sqlx.DB) (*MultiDBTx, error) {
	tx1, err := db1.Beginx()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction for db1: %w", err)
	}

	tx2, err := db2.Beginx()
	if err != nil {
		_ = tx1.Rollback() // tx2のエラー時にtx1をロールバック
		return nil, fmt.Errorf("failed to begin transaction for db2: %w", err)
	}

	return &MultiDBTx{tx1: tx1, tx2: tx2}, nil
}

// Commit は両方のトランザクションをコミットします
func (m *MultiDBTx) Commit() error {
	if err := m.tx1.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction for db1: %w", err)
	}
	if err := m.tx2.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction for db2: %w", err)
	}
	return nil
}

// Rollback は両方のトランザクションをロールバックします
func (m *MultiDBTx) Rollback() error {
	var err1, err2 error
	if m.tx1 != nil {
		err1 = m.tx1.Rollback()
	}
	if m.tx2 != nil {
		err2 = m.tx2.Rollback()
	}

	if err1 != nil || err2 != nil {
		return fmt.Errorf("failed to rollback transactions: db1 error: %v, db2 error: %v", err1, err2)
	}
	return nil
}

// ExecContext は指定したデータベーストランザクションでクエリを実行します
func (m *MultiDBTx) ExecContext(ctx context.Context, dbName string, query string, args ...interface{}) (sql.Result, error) {
	switch dbName {
	case "db1":
		return m.tx1.ExecContext(ctx, query, args...)
	case "db2":
		return m.tx2.ExecContext(ctx, query, args...)
	default:
		return m.tx1.ExecContext(ctx, query, args...)
	}
}

// GetContext は指定したデータベーストランザクションでデータを取得します
func (m *MultiDBTx) GetContext(ctx context.Context, dbName string, dest interface{}, query string, args ...interface{}) error {
	switch dbName {
	case "db1":
		return m.tx1.GetContext(ctx, dest, query, args...)
	case "db2":
		return m.tx2.GetContext(ctx, dest, query, args...)
	default:
		return m.tx1.GetContext(ctx, dest, query, args...)
	}
}

// SelectContext は指定したデータベーストランザクションでデータを取得します
func (m *MultiDBTx) SelectContext(ctx context.Context, dbName string, dest interface{}, query string, args ...interface{}) error {
	switch dbName {
	case "db1":
		return m.tx1.SelectContext(ctx, dest, query, args...)
	case "db2":
		return m.tx2.SelectContext(ctx, dest, query, args...)
	default:
		return m.tx1.SelectContext(ctx, dest, query, args...)
	}

}

type postInitializeRequest struct {
	PaymentServer string `json:"payment_server"`
}

type postInitializeResponse struct {
	Language string `json:"language"`
}

type apiHandler struct {
	db                *sqlx.DB
	db2               *sqlx.DB
	paymentGatewayURL string
}

func newHandler(db *sqlx.DB, db2 *sqlx.DB) *apiHandler {
	return &apiHandler{
		db:  db,
		db2: db2,
		// dummy
		paymentGatewayURL: "http://localhost:12345",
	}
}

func (h *apiHandler) postInitialize(w http.ResponseWriter, r *http.Request) {
	go func() {
		if _, err := http.Get("https://p.isu.ikura-hamu.work/api/group/collect"); err != nil {
			log.Printf("failed to communicate with pprotein: %v", err)
		}
	}()

	ctx := r.Context()
	req := &postInitializeRequest{}
	if err := bindJSON(r, req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if out, err := exec.Command("../sql/init.sh").CombinedOutput(); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("failed to initialize: %s: %w", string(out), err))
		return
	}

	if _, err := h.db.ExecContext(ctx, "UPDATE settings SET value = ? WHERE name = 'payment_gateway_url'", req.PaymentServer); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	h.paymentGatewayURL = req.PaymentServer

	// サーバー3に dbInitialize をリクエスト
	if err := forwardDbInitializeRequest(req.PaymentServer); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("failed to forward to dbInitialize: %w", err))
		return
	}

	// サーバー2に dbInitialize をリクエスト
	if err := forwardDbInitializeRequest2(req.PaymentServer); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("failed to forward to dbInitialize: %w", err))
		return
	}

	writeJSON(w, http.StatusOK, postInitializeResponse{Language: "go"})
}

func forwardDbInitializeRequest(paymentServer string) error {
	forwardRequest := postInitializeRequest{
		PaymentServer: paymentServer,
	}
	body, err := json.Marshal(forwardRequest)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := http.Post("http://192.168.0.13:8080/api/db/initialize", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to send request to dbInitialize: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("dbInitialize returned non-200 status: %d, body: %s", resp.StatusCode, string(responseBody))
	}

	return nil
}

func forwardDbInitializeRequest2(paymentServer string) error {
	forwardRequest := postInitializeRequest{
		PaymentServer: paymentServer,
	}
	body, err := json.Marshal(forwardRequest)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := http.Post("http://192.168.0.12:8080/api/db/initialize", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to send request to dbInitialize: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("dbInitialize returned non-200 status: %d, body: %s", resp.StatusCode, string(responseBody))
	}

	return nil
}

func (h *apiHandler) dbInitialize(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req := &postInitializeRequest{}
	if err := bindJSON(r, req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if out, err := exec.Command("../sql/init.sh").CombinedOutput(); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("failed to initialize: %s: %w", string(out), err))
		return
	}

	if _, err := h.db.ExecContext(ctx, "UPDATE settings SET value = ? WHERE name = 'payment_gateway_url'", req.PaymentServer); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	h.paymentGatewayURL = req.PaymentServer

	writeJSON(w, http.StatusOK, postInitializeResponse{Language: "go"})
}

type Coordinate struct {
	Latitude  int `json:"latitude"`
	Longitude int `json:"longitude"`
}

func bindJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func writeJSON(w http.ResponseWriter, statusCode int, v interface{}) {
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	buf, err := json.Marshal(v)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(statusCode)
	w.Write(buf)
}

func writeError(w http.ResponseWriter, statusCode int, err error) {
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
	w.WriteHeader(statusCode)
	buf, marshalError := json.Marshal(map[string]string{"message": err.Error()})
	if marshalError != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"marshaling error failed"}`))
		return
	}
	w.Write(buf)

	slog.Error("error response wrote", err)
}

func secureRandomStr(b int) string {
	k := make([]byte, b)
	if _, err := crand.Read(k); err != nil {
		panic(err)
	}
	return fmt.Sprintf("%x", k)
}
