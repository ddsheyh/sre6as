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
		prometheus.CounterOpts{Name: "chat_http_requests_total", Help: "Total HTTP requests"},
		[]string{"method", "endpoint", "status"},
	)
)

func init() {
	prometheus.MustRegister(httpRequests)
}

func jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, status int, message string) {
	jsonResponse(w, status, map[string]string{"error": message})
}

func extractUser(r *http.Request) (int, string, error) {
	auth := r.Header.Get("Authorization")
	if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
		return 0, "", fmt.Errorf("no token")
	}
	tokenStr := strings.TrimPrefix(auth, "Bearer ")
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return 0, "", fmt.Errorf("invalid token")
	}
	claims := token.Claims.(jwt.MapClaims)
	userID := int(claims["user_id"].(float64))
	email := claims["email"].(string)
	return userID, email, nil
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if err := db.Ping(); err != nil {
		jsonError(w, 503, "database unavailable")
		return
	}
	jsonResponse(w, 200, map[string]string{"status": "ok", "service": "chat-service"})
}

func messagesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getMessages(w, r)
	case http.MethodPost:
		sendMessage(w, r)
	default:
		jsonError(w, 405, "method not allowed")
	}
}

func getMessages(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, user_id, username, content, created_at FROM messages ORDER BY created_at DESC LIMIT 50")
	if err != nil {
		log.Printf("ERROR list messages: %v", err)
		httpRequests.WithLabelValues("GET", "/api/messages", "500").Inc()
		jsonError(w, 500, "internal error")
		return
	}
	defer rows.Close()

	type Message struct {
		ID        int    `json:"id"`
		UserID    int    `json:"user_id"`
		Username  string `json:"username"`
		Content   string `json:"content"`
		CreatedAt string `json:"created_at"`
	}
	messages := []Message{}
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.UserID, &m.Username, &m.Content, &m.CreatedAt); err != nil {
			continue
		}
		messages = append(messages, m)
	}
	httpRequests.WithLabelValues("GET", "/api/messages", "200").Inc()
	jsonResponse(w, 200, messages)
}

func sendMessage(w http.ResponseWriter, r *http.Request) {
	userID, email, err := extractUser(r)
	if err != nil {
		httpRequests.WithLabelValues("POST", "/api/messages", "401").Inc()
		jsonError(w, 401, "unauthorized")
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Content == "" {
		httpRequests.WithLabelValues("POST", "/api/messages", "400").Inc()
		jsonError(w, 400, "content is required")
		return
	}

	var id int
	err = db.QueryRow(
		"INSERT INTO messages (user_id, username, content) VALUES ($1, $2, $3) RETURNING id",
		userID, email, req.Content,
	).Scan(&id)
	if err != nil {
		log.Printf("ERROR send message: %v", err)
		httpRequests.WithLabelValues("POST", "/api/messages", "500").Inc()
		jsonError(w, 500, "failed to send message")
		return
	}

	httpRequests.WithLabelValues("POST", "/api/messages", "201").Inc()
	jsonResponse(w, 201, map[string]interface{}{"id": id, "status": "sent"})
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting chat-service...")

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
	http.HandleFunc("/api/messages", messagesHandler)
	http.HandleFunc("/api/messages/", messagesHandler)

	port := getEnv("SERVICE_PORT", "8080")
	log.Printf("chat-service listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
