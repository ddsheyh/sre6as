package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
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
		prometheus.CounterOpts{Name: "auth_http_requests_total", Help: "Total HTTP requests"},
		[]string{"method", "endpoint", "status"},
	)
	httpDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "auth_http_duration_seconds", Help: "HTTP request duration"},
		[]string{"method", "endpoint"},
	)
)

func init() {
	prometheus.MustRegister(httpRequests)
	prometheus.MustRegister(httpDuration)
}

func hashPassword(password string) string {
	h := sha256.Sum256([]byte(password))
	return hex.EncodeToString(h[:])
}

func generateToken(userID int, email string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"email":   email,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
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
	jsonResponse(w, 200, map[string]string{"status": "ok", "service": "auth-service"})
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		httpDuration.WithLabelValues(r.Method, "/api/auth/register").Observe(time.Since(start).Seconds())
	}()

	if r.Method != http.MethodPost {
		httpRequests.WithLabelValues(r.Method, "/api/auth/register", "405").Inc()
		jsonError(w, 405, "method not allowed")
		return
	}

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpRequests.WithLabelValues(r.Method, "/api/auth/register", "400").Inc()
		jsonError(w, 400, "invalid request body")
		return
	}
	if req.Email == "" || req.Password == "" || req.Name == "" {
		httpRequests.WithLabelValues(r.Method, "/api/auth/register", "400").Inc()
		jsonError(w, 400, "email, password and name are required")
		return
	}

	passwordHash := hashPassword(req.Password)
	var userID int
	err := db.QueryRow(
		"INSERT INTO users (email, password_hash, name) VALUES ($1, $2, $3) RETURNING id",
		req.Email, passwordHash, req.Name,
	).Scan(&userID)
	if err != nil {
		log.Printf("ERROR register: %v", err)
		httpRequests.WithLabelValues(r.Method, "/api/auth/register", "409").Inc()
		jsonError(w, 409, "user already exists or database error")
		return
	}

	token, err := generateToken(userID, req.Email)
	if err != nil {
		log.Printf("ERROR token generation: %v", err)
		httpRequests.WithLabelValues(r.Method, "/api/auth/register", "500").Inc()
		jsonError(w, 500, "internal server error")
		return
	}

	httpRequests.WithLabelValues(r.Method, "/api/auth/register", "201").Inc()
	jsonResponse(w, 201, map[string]interface{}{
		"token":   token,
		"user_id": userID,
		"name":    req.Name,
	})
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		httpDuration.WithLabelValues(r.Method, "/api/auth/login").Observe(time.Since(start).Seconds())
	}()

	if r.Method != http.MethodPost {
		httpRequests.WithLabelValues(r.Method, "/api/auth/login", "405").Inc()
		jsonError(w, 405, "method not allowed")
		return
	}

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpRequests.WithLabelValues(r.Method, "/api/auth/login", "400").Inc()
		jsonError(w, 400, "invalid request body")
		return
	}

	passwordHash := hashPassword(req.Password)
	var userID int
	var name string
	err := db.QueryRow(
		"SELECT id, name FROM users WHERE email = $1 AND password_hash = $2",
		req.Email, passwordHash,
	).Scan(&userID, &name)
	if err != nil {
		log.Printf("ERROR login: %v", err)
		httpRequests.WithLabelValues(r.Method, "/api/auth/login", "401").Inc()
		jsonError(w, 401, "invalid email or password")
		return
	}

	token, err := generateToken(userID, req.Email)
	if err != nil {
		log.Printf("ERROR token generation: %v", err)
		httpRequests.WithLabelValues(r.Method, "/api/auth/login", "500").Inc()
		jsonError(w, 500, "internal server error")
		return
	}

	httpRequests.WithLabelValues(r.Method, "/api/auth/login", "200").Inc()
	jsonResponse(w, 200, map[string]interface{}{
		"token":   token,
		"user_id": userID,
		"name":    name,
	})
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting auth-service...")

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
	http.HandleFunc("/api/auth/register", registerHandler)
	http.HandleFunc("/api/auth/login", loginHandler)

	port := getEnv("SERVICE_PORT", "8080")
	log.Printf("auth-service listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
