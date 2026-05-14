package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var db *sql.DB

var (
	httpRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "notification_http_requests_total", Help: "Total HTTP requests"},
		[]string{"method", "endpoint", "status"},
	)
	httpDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "notification_http_duration_seconds", Help: "HTTP request duration"},
		[]string{"method", "endpoint"},
	)
	notificationsSent = prometheus.NewCounter(
		prometheus.CounterOpts{Name: "notifications_sent_total", Help: "Total notifications sent"},
	)
)

func init() {
	prometheus.MustRegister(httpRequests)
	prometheus.MustRegister(httpDuration)
	prometheus.MustRegister(notificationsSent)
}

func jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, status int, message string) {
	jsonResponse(w, status, map[string]string{"error": message})
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if err := db.Ping(); err != nil {
		jsonError(w, 503, "database unavailable")
		return
	}
	jsonResponse(w, 200, map[string]string{"status": "ok", "service": "notification-service"})
}

func notificationsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		sendNotification(w, r)
	case http.MethodGet:
		listNotifications(w, r)
	default:
		httpRequests.WithLabelValues(r.Method, "/api/notifications", "405").Inc()
		jsonError(w, 405, "method not allowed")
	}
}

func sendNotification(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		httpDuration.WithLabelValues("POST", "/api/notifications").Observe(time.Since(start).Seconds())
	}()

	var req struct {
		UserID  int    `json:"user_id"`
		Type    string `json:"type"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpRequests.WithLabelValues("POST", "/api/notifications", "400").Inc()
		jsonError(w, 400, "invalid request body")
		return
	}
	if req.Message == "" {
		req.Message = "You have a new notification"
	}
	if req.Type == "" {
		req.Type = "info"
	}

	var id int
	err := db.QueryRow(
		"INSERT INTO notifications (user_id, type, message) VALUES ($1, $2, $3) RETURNING id",
		req.UserID, req.Type, req.Message,
	).Scan(&id)
	if err != nil {
		log.Printf("ERROR send notification: %v", err)
		httpRequests.WithLabelValues("POST", "/api/notifications", "500").Inc()
		jsonError(w, 500, "failed to create notification")
		return
	}

	notificationsSent.Inc()
	log.Printf("Notification sent: id=%d user=%d type=%s", id, req.UserID, req.Type)

	httpRequests.WithLabelValues("POST", "/api/notifications", "201").Inc()
	jsonResponse(w, 201, map[string]interface{}{
		"id":      id,
		"user_id": req.UserID,
		"type":    req.Type,
		"status":  "sent",
	})
}

func listNotifications(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		httpDuration.WithLabelValues("GET", "/api/notifications").Observe(time.Since(start).Seconds())
	}()

	rows, err := db.Query("SELECT id, user_id, type, message, status, created_at FROM notifications ORDER BY created_at DESC LIMIT 50")
	if err != nil {
		log.Printf("ERROR list notifications: %v", err)
		httpRequests.WithLabelValues("GET", "/api/notifications", "500").Inc()
		jsonError(w, 500, "internal error")
		return
	}
	defer rows.Close()

	type Notification struct {
		ID        int    `json:"id"`
		UserID    int    `json:"user_id"`
		Type      string `json:"type"`
		Message   string `json:"message"`
		Status    string `json:"status"`
		CreatedAt string `json:"created_at"`
	}

	notifications := []Notification{}
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Message, &n.Status, &n.CreatedAt); err != nil {
			continue
		}
		notifications = append(notifications, n)
	}

	httpRequests.WithLabelValues("GET", "/api/notifications", "200").Inc()
	jsonResponse(w, 200, notifications)
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting notification-service...")

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		getEnv("DB_HOST", "postgres"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_USER", "goticket"),
		getEnv("DB_PASSWORD", "goticket123"),
		getEnv("DB_NAME", "goticket"),
	)

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
		log.Fatalf("FATAL: cannot connect to database: %v", err)
	}
	log.Println("Connected to database")

	http.HandleFunc("/health", healthHandler)
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/api/notifications", notificationsHandler)
	http.HandleFunc("/api/notifications/", notificationsHandler)

	port := getEnv("SERVICE_PORT", "8080")
	log.Printf("notification-service listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
