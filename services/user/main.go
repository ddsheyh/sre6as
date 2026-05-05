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
		prometheus.CounterOpts{Name: "user_http_requests_total", Help: "Total HTTP requests"},
		[]string{"method", "endpoint", "status"},
	)
	httpDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "user_http_duration_seconds", Help: "HTTP request duration"},
		[]string{"method", "endpoint"},
	)
)

func init() {
	prometheus.MustRegister(httpRequests)
	prometheus.MustRegister(httpDuration)
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
		jsonError(w, 503, "database unavailable")
		return
	}
	jsonResponse(w, 200, map[string]string{"status": "ok", "service": "user-service"})
}

func meHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		httpDuration.WithLabelValues(r.Method, "/api/users/me").Observe(time.Since(start).Seconds())
	}()

	if r.Method != http.MethodGet {
		httpRequests.WithLabelValues(r.Method, "/api/users/me", "405").Inc()
		jsonError(w, 405, "method not allowed")
		return
	}

	userID, err := extractUserID(r)
	if err != nil {
		httpRequests.WithLabelValues(r.Method, "/api/users/me", "401").Inc()
		jsonError(w, 401, "unauthorized")
		return
	}

	var email, name string
	var createdAt time.Time
	err = db.QueryRow("SELECT email, name, created_at FROM users WHERE id = $1", userID).
		Scan(&email, &name, &createdAt)
	if err != nil {
		log.Printf("ERROR get user: %v", err)
		httpRequests.WithLabelValues(r.Method, "/api/users/me", "404").Inc()
		jsonError(w, 404, "user not found")
		return
	}

	httpRequests.WithLabelValues(r.Method, "/api/users/me", "200").Inc()
	jsonResponse(w, 200, map[string]interface{}{
		"id":         userID,
		"email":      email,
		"name":       name,
		"created_at": createdAt,
	})
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting user-service...")

	jwtSecret = []byte(getEnv("JWT_SECRET", "goticket-secret-key"))

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
	http.HandleFunc("/api/users/me", meHandler)

	port := getEnv("SERVICE_PORT", "8080")
	log.Printf("user-service listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
