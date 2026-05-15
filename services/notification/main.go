package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var collection *mongo.Collection

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

// Notification represents a notification stored in MongoDB
type Notification struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID    int                `bson:"user_id" json:"user_id"`
	Type      string             `bson:"type" json:"type"`
	Message   string             `bson:"message" json:"message"`
	Status    string             `bson:"status" json:"status"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
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
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := collection.Database().Client().Ping(ctx, nil); err != nil {
		jsonError(w, 503, "mongodb unavailable")
		return
	}
	jsonResponse(w, 200, map[string]string{"status": "ok", "service": "notification-service", "database": "mongodb"})
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	notif := Notification{
		UserID:    req.UserID,
		Type:      req.Type,
		Message:   req.Message,
		Status:    "sent",
		CreatedAt: time.Now(),
	}

	result, err := collection.InsertOne(ctx, notif)
	if err != nil {
		log.Printf("ERROR send notification: %v", err)
		httpRequests.WithLabelValues("POST", "/api/notifications", "500").Inc()
		jsonError(w, 500, "failed to create notification")
		return
	}

	notificationsSent.Inc()
	log.Printf("Notification sent: id=%v user=%d type=%s", result.InsertedID, req.UserID, req.Type)

	httpRequests.WithLabelValues("POST", "/api/notifications", "201").Inc()
	jsonResponse(w, 201, map[string]interface{}{
		"id":      result.InsertedID,
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetLimit(50)
	cursor, err := collection.Find(ctx, bson.D{}, opts)
	if err != nil {
		log.Printf("ERROR list notifications: %v", err)
		httpRequests.WithLabelValues("GET", "/api/notifications", "500").Inc()
		jsonError(w, 500, "internal error")
		return
	}
	defer cursor.Close(ctx)

	var notifications []Notification
	if err := cursor.All(ctx, &notifications); err != nil {
		log.Printf("ERROR decode notifications: %v", err)
		httpRequests.WithLabelValues("GET", "/api/notifications", "500").Inc()
		jsonError(w, 500, "internal error")
		return
	}
	if notifications == nil {
		notifications = []Notification{}
	}

	httpRequests.WithLabelValues("GET", "/api/notifications", "200").Inc()
	jsonResponse(w, 200, notifications)
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting notification-service...")

	mongoURI := getEnv("MONGO_URI", "mongodb://mongo:27017")
	mongoDBName := getEnv("MONGO_DB", "goticket")

	// Connect to MongoDB with retry
	var client *mongo.Client
	var err error
	for i := 0; i < 30; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		client, err = mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
		if err == nil {
			err = client.Ping(ctx, nil)
		}
		cancel()
		if err == nil {
			break
		}
		log.Printf("Waiting for MongoDB... attempt %d/30: %v", i+1, err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatalf("FATAL: cannot connect to MongoDB: %v", err)
	}
	log.Println("Connected to MongoDB")

	collection = client.Database(mongoDBName).Collection("notifications")

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
