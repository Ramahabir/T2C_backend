package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/bcrypt"
)

var jwtSecret = []byte("your-secret-key-change-in-production")

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Response represents a standard API response
type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// SetupRouter creates and configures the Chi router
func SetupRouter() *chi.Mux {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check
	r.Get("/api/health", healthCheck)

	// Authentication routes
	r.Route("/api/auth", func(r chi.Router) {
		r.Post("/login", login)
		r.Post("/qr-login", generateQRLogin)
		r.Post("/verify-token", verifyToken)
		r.Post("/logout", logout)
		r.Post("/register", register)
	})

	// Session management routes (public for station use)
	r.Post("/api/request-session", requestSession)
	r.Post("/api/check-session", checkSession)
	r.Post("/api/connect-session", connectSession)
	r.Post("/api/end-session", endSession)

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware)

		// User management
		r.Get("/api/user/profile", getUserProfile)
		r.Get("/api/user/stats", getUserStats)
		r.Put("/api/user/profile", updateUserProfile)

		// Transactions
		r.Get("/api/transactions", getTransactions)
		r.Post("/api/transactions", createTransaction)
		r.Get("/api/transactions/{id}", getTransactionDetail)

		// Redemptions
		r.Get("/api/redemption/options", getRedemptionOptions)
		r.Post("/api/redemption/redeem", redeemPoints)
		r.Get("/api/redemption/history", getRedemptionHistory)

		// Station management
		r.Get("/api/station/status", getStationStatus)
		r.Post("/api/station/deposit", processDeposit)
		r.Get("/api/station/config", getStationConfig)

		// Session-based deposit (requires auth)
		r.Post("/api/deposit", deposit)
	})

	// WebSocket
	r.Get("/ws", handleWebSocket)

	return r
}

// authMiddleware validates JWT tokens
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString := r.Header.Get("Authorization")
		if tokenString == "" {
			respondJSON(w, http.StatusUnauthorized, Response{
				Success: false,
				Error:   "Authorization token required",
			})
			return
		}

		// Remove "Bearer " prefix if present
		if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
			tokenString = tokenString[7:]
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			respondJSON(w, http.StatusUnauthorized, Response{
				Success: false,
				Error:   "Invalid token",
			})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			respondJSON(w, http.StatusUnauthorized, Response{
				Success: false,
				Error:   "Invalid token claims",
			})
			return
		}

		// Add user ID to context
		r.Header.Set("X-User-ID", claims["user_id"].(string))
		next.ServeHTTP(w, r)
	})
}

// Helper functions
func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}

func getUserIDFromHeader(r *http.Request) (int, error) {
	userIDStr := r.Header.Get("X-User-ID")
	return strconv.Atoi(userIDStr)
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func checkPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func generateJWT(userID int) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": strconv.Itoa(userID),
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
	})
	return token.SignedString(jwtSecret)
}

func verifyJWT(tokenString string) (int, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})

	if err != nil || !token.Valid {
		return 0, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, jwt.ErrInvalidKey
	}

	userIDStr, ok := claims["user_id"].(string)
	if !ok {
		return 0, jwt.ErrInvalidKey
	}

	return strconv.Atoi(userIDStr)
}

// Health check handler
func healthCheck(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Message: "API is healthy",
		Data: map[string]interface{}{
			"status":    "ok",
			"timestamp": time.Now(),
		},
	})
}

// WebSocket handler
func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if err := conn.WriteMessage(messageType, p); err != nil {
			return
		}
	}
}
