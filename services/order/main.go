package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var db *sql.DB
var jwtSecret []byte

var (
	httpRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "order_http_requests_total", Help: "Total HTTP requests"},
		[]string{"method", "endpoint", "status"},
	)
	httpDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "order_http_duration_seconds", Help: "HTTP request duration"},
		[]string{"method", "endpoint"},
	)
	orderErrors = prometheus.NewCounter(
		prometheus.CounterOpts{Name: "order_errors_total", Help: "Total order processing errors"},
	)
	dbUp = prometheus.NewGauge(
		prometheus.GaugeOpts{Name: "order_db_up", Help: "Database connection status (1=up, 0=down)"},
	)
)

func init() {
	prometheus.MustRegister(httpRequests)
	prometheus.MustRegister(httpDuration)
	prometheus.MustRegister(orderErrors)
	prometheus.MustRegister(dbUp)
}

func jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, status int, message string) {
	jsonResponse(w, status, map[string]string{"error": message})
}

func extractUserID(r *http.Request) (int, error) {
	auth := r.Header.Get("Authorization")
	if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
		return 0, fmt.Errorf("no token")
	}
	tokenStr := strings.TrimPrefix(auth, "Bearer ")
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return 0, fmt.Errorf("invalid token")
	}
	claims := token.Claims.(jwt.MapClaims)
	userID := int(claims["user_id"].(float64))
	return userID, nil
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if err := db.Ping(); err != nil {
		log.Printf("HEALTH CHECK FAILED: %v", err)
		dbUp.Set(0)
		jsonError(w, 503, "database unavailable: "+err.Error())
		return
	}
	dbUp.Set(1)
	jsonResponse(w, 200, map[string]string{"status": "ok", "service": "order-service"})
}

func ordersHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	switch r.Method {
	case http.MethodPost:
		createOrder(w, r, start)
	case http.MethodGet:
		listOrders(w, r, start)
	default:
		httpRequests.WithLabelValues(r.Method, "/api/orders", "405").Inc()
		jsonError(w, 405, "method not allowed")
	}
}

func createOrder(w http.ResponseWriter, r *http.Request, start time.Time) {
	defer func() {
		httpDuration.WithLabelValues("POST", "/api/orders").Observe(time.Since(start).Seconds())
	}()

	userID, err := extractUserID(r)
	if err != nil {
		httpRequests.WithLabelValues("POST", "/api/orders", "401").Inc()
		jsonError(w, 401, "unauthorized")
		return
	}

	var req struct {
		EventID  int `json:"event_id"`
		Quantity int `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpRequests.WithLabelValues("POST", "/api/orders", "400").Inc()
		jsonError(w, 400, "invalid request body")
		return
	}
	if req.Quantity <= 0 {
		req.Quantity = 1
	}

	var price float64
	var available int
	err = db.QueryRow("SELECT price, available_tickets FROM events WHERE id = $1", req.EventID).
		Scan(&price, &available)
	if err != nil {
		log.Printf("ERROR get event for order: %v", err)
		orderErrors.Inc()
		httpRequests.WithLabelValues("POST", "/api/orders", "404").Inc()
		jsonError(w, 404, "event not found")
		return
	}
	if available < req.Quantity {
		httpRequests.WithLabelValues("POST", "/api/orders", "400").Inc()
		jsonError(w, 400, "not enough tickets available")
		return
	}

	totalPrice := price * float64(req.Quantity)

	var orderID int
	err = db.QueryRow(
		"INSERT INTO orders (user_id, event_id, quantity, total_price, status) VALUES ($1, $2, $3, $4, 'confirmed') RETURNING id",
		userID, req.EventID, req.Quantity, totalPrice,
	).Scan(&orderID)
	if err != nil {
		log.Printf("ERROR create order: %v", err)
		orderErrors.Inc()
		httpRequests.WithLabelValues("POST", "/api/orders", "500").Inc()
		jsonError(w, 500, "failed to create order")
		return
	}

	_, err = db.Exec("UPDATE events SET available_tickets = available_tickets - $1 WHERE id = $2", req.Quantity, req.EventID)
	if err != nil {
		log.Printf("ERROR update tickets: %v", err)
	}

	httpRequests.WithLabelValues("POST", "/api/orders", "201").Inc()
	jsonResponse(w, 201, map[string]interface{}{
		"order_id":    orderID,
		"event_id":    req.EventID,
		"quantity":    req.Quantity,
		"total_price": totalPrice,
		"status":      "confirmed",
	})
}

func listOrders(w http.ResponseWriter, r *http.Request, start time.Time) {
	defer func() {
		httpDuration.WithLabelValues("GET", "/api/orders").Observe(time.Since(start).Seconds())
	}()

	userID, err := extractUserID(r)
	if err != nil {
		httpRequests.WithLabelValues("GET", "/api/orders", "401").Inc()
		jsonError(w, 401, "unauthorized")
		return
	}

	rows, err := db.Query(`
		SELECT o.id, o.event_id, e.title, o.quantity, o.total_price, o.status, o.created_at
		FROM orders o JOIN events e ON o.event_id = e.id
		WHERE o.user_id = $1 ORDER BY o.created_at DESC`, userID)
	if err != nil {
		log.Printf("ERROR list orders: %v", err)
		orderErrors.Inc()
		httpRequests.WithLabelValues("GET", "/api/orders", "500").Inc()
		jsonError(w, 500, "failed to list orders")
		return
	}
	defer rows.Close()

	type Order struct {
		ID         int     `json:"id"`
		EventID    int     `json:"event_id"`
		EventTitle string  `json:"event_title"`
		Quantity   int     `json:"quantity"`
		TotalPrice float64 `json:"total_price"`
		Status     string  `json:"status"`
		CreatedAt  string  `json:"created_at"`
	}

	orders := []Order{}
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.EventID, &o.EventTitle, &o.Quantity, &o.TotalPrice, &o.Status, &o.CreatedAt); err != nil {
			log.Printf("ERROR scan order: %v", err)
			continue
		}
		orders = append(orders, o)
	}

	httpRequests.WithLabelValues("GET", "/api/orders", "200").Inc()
	jsonResponse(w, 200, orders)
}

func startHealthChecker() {
	go func() {
		for {
			if err := db.Ping(); err != nil {
				dbUp.Set(0)
				log.Printf("DB HEALTH CHECK FAILED: %v", err)
			} else {
				dbUp.Set(1)
			}
			time.Sleep(10 * time.Second)
		}
	}()
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting order-service...")

	jwtSecret = []byte(getEnv("JWT_SECRET", "goticket-secret-key"))

	dbHost := getEnv("DB_HOST", "postgres")
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost,
		getEnv("DB_PORT", "5432"),
		getEnv("DB_USER", "goticket"),
		getEnv("DB_PASSWORD", "goticket123"),
		getEnv("DB_NAME", "goticket"),
	)

	log.Printf("Connecting to database at host: %s", dbHost)

	var err error
	for i := 0; i < 30; i++ {
		db, err = sql.Open("postgres", connStr)
		if err == nil {
			err = db.Ping()
		}
		if err == nil {
			break
		}
		log.Printf("Waiting for database... attempt %d/30: %v", i+1, err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Printf("WARNING: cannot connect to database: %v", err)
		log.Println("Service will start but database operations will fail")
		dbUp.Set(0)
	} else {
		log.Println("Connected to database")
		dbUp.Set(1)
	}

	startHealthChecker()

	http.HandleFunc("/health", healthHandler)
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/api/orders", ordersHandler)
	http.HandleFunc("/api/orders/", ordersHandler)

	port := getEnv("SERVICE_PORT", "8080")
	log.Printf("order-service listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
