package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var collection *mongo.Collection
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

// Message represents a chat message stored in MongoDB
type Message struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID    int                `bson:"user_id" json:"user_id"`
	Username  string             `bson:"username" json:"username"`
	Content   string             `bson:"content" json:"content"`
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
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := collection.Database().Client().Ping(ctx, nil); err != nil {
		jsonError(w, 503, "mongodb unavailable")
		return
	}
	jsonResponse(w, 200, map[string]string{"status": "ok", "service": "chat-service", "database": "mongodb"})
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetLimit(50)
	cursor, err := collection.Find(ctx, bson.D{}, opts)
	if err != nil {
		log.Printf("ERROR list messages: %v", err)
		httpRequests.WithLabelValues("GET", "/api/messages", "500").Inc()
		jsonError(w, 500, "internal error")
		return
	}
	defer cursor.Close(ctx)

	var messages []Message
	if err := cursor.All(ctx, &messages); err != nil {
		log.Printf("ERROR decode messages: %v", err)
		httpRequests.WithLabelValues("GET", "/api/messages", "500").Inc()
		jsonError(w, 500, "internal error")
		return
	}
	if messages == nil {
		messages = []Message{}
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msg := Message{
		UserID:    userID,
		Username:  email,
		Content:   req.Content,
		CreatedAt: time.Now(),
	}

	result, err := collection.InsertOne(ctx, msg)
	if err != nil {
		log.Printf("ERROR send message: %v", err)
		httpRequests.WithLabelValues("POST", "/api/messages", "500").Inc()
		jsonError(w, 500, "failed to send message")
		return
	}

	httpRequests.WithLabelValues("POST", "/api/messages", "201").Inc()
	jsonResponse(w, 201, map[string]interface{}{"id": result.InsertedID, "status": "sent"})
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting chat-service...")

	jwtSecret = []byte(getEnv("JWT_SECRET", "goticket-secret-key"))

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

	collection = client.Database(mongoDBName).Collection("messages")

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
