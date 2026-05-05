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

	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var db *sql.DB

var (
	httpRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "event_http_requests_total", Help: "Total HTTP requests"},
		[]string{"method", "endpoint", "status"},
	)
	httpDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "event_http_duration_seconds", Help: "HTTP request duration"},
		[]string{"method", "endpoint"},
	)
)

func init() {
	prometheus.MustRegister(httpRequests)
	prometheus.MustRegister(httpDuration)
}

type Event struct {
	ID               int     `json:"id"`
	Title            string  `json:"title"`
	Description      string  `json:"description"`
	Location         string  `json:"location"`
	EventDate        string  `json:"event_date"`
	Price            float64 `json:"price"`
	AvailableTickets int     `json:"available_tickets"`
	ImageURL         string  `json:"image_url"`
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
	jsonResponse(w, 200, map[string]string{"status": "ok", "service": "event-service"})
}

func eventsHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	if r.Method != http.MethodGet {
		httpRequests.WithLabelValues(r.Method, "/api/events", "405").Inc()
		jsonError(w, 405, "method not allowed")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/events")
	path = strings.TrimPrefix(path, "/")

	if path != "" {
		defer func() {
			httpDuration.WithLabelValues(r.Method, "/api/events/{id}").Observe(time.Since(start).Seconds())
		}()
		var e Event
		err := db.QueryRow(
			"SELECT id, title, description, location, event_date, price, available_tickets, image_url FROM events WHERE id = $1",
			path,
		).Scan(&e.ID, &e.Title, &e.Description, &e.Location, &e.EventDate, &e.Price, &e.AvailableTickets, &e.ImageURL)
		if err != nil {
			log.Printf("ERROR get event: %v", err)
			httpRequests.WithLabelValues(r.Method, "/api/events/{id}", "404").Inc()
			jsonError(w, 404, "event not found")
			return
		}
		httpRequests.WithLabelValues(r.Method, "/api/events/{id}", "200").Inc()
		jsonResponse(w, 200, e)
		return
	}

	defer func() {
		httpDuration.WithLabelValues(r.Method, "/api/events").Observe(time.Since(start).Seconds())
	}()
	rows, err := db.Query("SELECT id, title, description, location, event_date, price, available_tickets, image_url FROM events ORDER BY event_date")
	if err != nil {
		log.Printf("ERROR list events: %v", err)
		httpRequests.WithLabelValues(r.Method, "/api/events", "500").Inc()
		jsonError(w, 500, "internal server error")
		return
	}
	defer rows.Close()

	events := []Event{}
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.Title, &e.Description, &e.Location, &e.EventDate, &e.Price, &e.AvailableTickets, &e.ImageURL); err != nil {
			log.Printf("ERROR scan event: %v", err)
			continue
		}
		events = append(events, e)
	}

	httpRequests.WithLabelValues(r.Method, "/api/events", "200").Inc()
	jsonResponse(w, 200, events)
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting event-service...")

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
	http.HandleFunc("/api/events", eventsHandler)
	http.HandleFunc("/api/events/", eventsHandler)

	port := getEnv("SERVICE_PORT", "8080")
	log.Printf("event-service listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
