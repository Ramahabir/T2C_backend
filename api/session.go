package api

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"t2cbackend/database"
	"time"

	"github.com/google/uuid"
	qrcode "github.com/skip2/go-qrcode"
)

// RequestSessionRequest represents a session request
type RequestSessionRequest struct {
	StationID string `json:"station_id,omitempty"`
}

// CheckSessionRequest represents a session check request
type CheckSessionRequest struct {
	SessionToken string `json:"sessionToken"`
}

// ConnectSessionRequest represents a connect session request
type ConnectSessionRequest struct {
	SessionToken string `json:"sessionToken"`
	AuthToken    string `json:"authToken"`
}

// EndSessionRequest represents an end session request
type EndSessionRequest struct {
	SessionToken string `json:"sessionToken"`
}

// SessionDepositRequest represents a deposit request during active session
type SessionDepositRequest struct {
	Material     string  `json:"material"`
	Weight       float64 `json:"weight"`
	SessionToken string  `json:"sessionToken"`
}

// requestSession creates a new station session and generates QR code
func requestSession(w http.ResponseWriter, r *http.Request) {
	var req RequestSessionRequest
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&req)
	}

	// Generate unique session token
	sessionToken := uuid.New().String()

	log.Printf("Creating new session: token=%s, station=%s", sessionToken, req.StationID)

	// Set expiration time (5 minutes from now)
	expiresAt := time.Now().Add(5 * time.Minute)

	// Insert session into database
	stationID := req.StationID
	if stationID == "" {
		stationID = "default"
	}

	_, err := database.DB.Exec(
		"INSERT INTO station_sessions (session_token, station_id, status, expires_at) VALUES (?, ?, ?, ?)",
		sessionToken, stationID, "pending", expiresAt,
	)
	if err != nil {
		log.Printf("Failed to insert session into database: %v", err)
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to create session",
		})
		return
	}

	// Generate QR code containing session token
	qrBytes, err := qrcode.Encode(sessionToken, qrcode.Medium, 256)
	if err != nil {
		log.Printf("Failed to generate QR code: %v", err)
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to generate QR code",
		})
		return
	}

	// Convert to base64
	qrBase64 := base64.StdEncoding.EncodeToString(qrBytes)

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Message: "Session token generated",
		Data: map[string]interface{}{
			"sessionToken": sessionToken,
			"qrCode":       "data:image/png;base64," + qrBase64,
			"expiresAt":    expiresAt.Format(time.RFC3339),
			"status":       "pending",
		},
	})
}

// checkSession checks the status of a station session
func checkSession(w http.ResponseWriter, r *http.Request) {
	var req CheckSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Invalid request body",
		})
		return
	}

	if req.SessionToken == "" {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Session token is required",
		})
		return
	}

	// Query session from database
	var sessionID int
	var userID *int
	var status string
	var authToken *string
	var expiresAt time.Time

	err := database.DB.QueryRow(
		"SELECT id, user_id, status, auth_token, expires_at FROM station_sessions WHERE session_token = ?",
		req.SessionToken,
	).Scan(&sessionID, &userID, &status, &authToken, &expiresAt)

	if err != nil {
		respondJSON(w, http.StatusNotFound, Response{
			Success: false,
			Error:   "Session not found",
		})
		return
	}

	// Check if session has expired
	if time.Now().After(expiresAt) {
		// Update session status to expired
		database.DB.Exec("UPDATE station_sessions SET status = ? WHERE id = ?", "expired", sessionID)

		respondJSON(w, http.StatusUnauthorized, Response{
			Success: false,
			Error:   "Session has expired",
		})
		return
	}

	// If session is still pending
	if status == "pending" || userID == nil {
		respondJSON(w, http.StatusOK, Response{
			Success: true,
			Data: map[string]interface{}{
				"status": "pending",
			},
		})
		return
	}

	// Session is connected - get user details
	var user database.User
	err = database.DB.QueryRow(
		"SELECT id, email, name, total_points FROM users WHERE id = ?",
		*userID,
	).Scan(&user.ID, &user.Email, &user.Name, &user.TotalPoints)

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to retrieve user details",
		})
		return
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"status":      status,
			"authToken":   authToken,
			"userId":      user.ID,
			"userName":    user.Name,
			"userBalance": user.TotalPoints,
		},
	})
}

// connectSession connects an authenticated user to a station session
func connectSession(w http.ResponseWriter, r *http.Request) {
	var req ConnectSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Invalid request body",
		})
		return
	}

	if req.SessionToken == "" || req.AuthToken == "" {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Session token and auth token are required",
		})
		return
	}

	// Verify auth token and get user ID
	userID, err := verifyJWT(req.AuthToken)
	if err != nil {
		respondJSON(w, http.StatusUnauthorized, Response{
			Success: false,
			Error:   "Invalid auth token",
		})
		return
	}

	// Check if session exists and is pending
	var sessionID int
	var status string
	var expiresAt time.Time
	var existingUserID *int

	err = database.DB.QueryRow(
		"SELECT id, user_id, status, expires_at FROM station_sessions WHERE session_token = ?",
		req.SessionToken,
	).Scan(&sessionID, &existingUserID, &status, &expiresAt)

	if err != nil {
		respondJSON(w, http.StatusNotFound, Response{
			Success: false,
			Error:   "Session not found",
		})
		return
	}

	// Check if session has expired
	if time.Now().After(expiresAt) {
		respondJSON(w, http.StatusUnauthorized, Response{
			Success: false,
			Error:   "Session has expired",
		})
		return
	}

	// Check if session is already connected
	if existingUserID != nil {
		respondJSON(w, http.StatusConflict, Response{
			Success: false,
			Error:   "Session is already connected to a user",
		})
		return
	}

	// Update session with user information
	_, err = database.DB.Exec(
		"UPDATE station_sessions SET user_id = ?, auth_token = ?, status = ? WHERE id = ?",
		userID, req.AuthToken, "connected", sessionID,
	)
	if err != nil {
		log.Printf("Failed to update session: %v", err)
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to connect session",
		})
		return
	}

	log.Printf("User %d connected to session %s", userID, req.SessionToken)

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Message: "User connected to station session",
	})
}

// endSession ends a station session
func endSession(w http.ResponseWriter, r *http.Request) {
	var req EndSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Invalid request body",
		})
		return
	}

	if req.SessionToken == "" {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Session token is required",
		})
		return
	}

	// Update session status to expired and set ended_at
	result, err := database.DB.Exec(
		"UPDATE station_sessions SET status = ?, ended_at = ? WHERE session_token = ?",
		"expired", time.Now(), req.SessionToken,
	)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to end session",
		})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		respondJSON(w, http.StatusNotFound, Response{
			Success: false,
			Error:   "Session not found",
		})
		return
	}

	log.Printf("Session ended: %s", req.SessionToken)

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Message: "Session ended",
	})
}

// deposit processes a deposit during an active session
func deposit(w http.ResponseWriter, r *http.Request) {
	// Get user ID from JWT token
	userID, err := getUserIDFromHeader(r)
	if err != nil {
		respondJSON(w, http.StatusUnauthorized, Response{
			Success: false,
			Error:   "Invalid user authentication",
		})
		return
	}

	var req SessionDepositRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Invalid request body",
		})
		return
	}

	// Validate input
	if req.Material == "" || req.Weight <= 0 || req.SessionToken == "" {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Material, weight, and session token are required",
		})
		return
	}

	// Verify session is active and linked to the user
	var sessionID int
	var sessionUserID *int
	var status string

	err = database.DB.QueryRow(
		"SELECT id, user_id, status FROM station_sessions WHERE session_token = ?",
		req.SessionToken,
	).Scan(&sessionID, &sessionUserID, &status)

	if err != nil {
		respondJSON(w, http.StatusNotFound, Response{
			Success: false,
			Error:   "Session not found",
		})
		return
	}

	// Verify session belongs to the authenticated user
	if sessionUserID == nil || *sessionUserID != userID {
		respondJSON(w, http.StatusForbidden, Response{
			Success: false,
			Error:   "Session not linked to this user",
		})
		return
	}

	// Calculate points based on material and weight
	points := CalculatePoints(req.Material, req.Weight)
	if points == 0 {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Invalid material type",
		})
		return
	}

	// Begin database transaction
	tx, err := database.DB.Begin()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to process deposit",
		})
		return
	}
	defer tx.Rollback()

	// Insert transaction record
	result, err := tx.Exec(`
		INSERT INTO transactions (user_id, type, item_type, weight, points_earned, station_id, session_token)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, userID, "deposit", req.Material, req.Weight, points, 1, req.SessionToken)

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to record transaction",
		})
		return
	}

	transactionID, _ := result.LastInsertId()

	// Update user points
	_, err = tx.Exec(
		"UPDATE users SET total_points = total_points + ?, updated_at = ? WHERE id = ?",
		points, time.Now(), userID,
	)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to update user balance",
		})
		return
	}

	// Update session status to active
	_, err = tx.Exec(
		"UPDATE station_sessions SET status = ? WHERE id = ?",
		"active", sessionID,
	)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to update session",
		})
		return
	}

	if err = tx.Commit(); err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to complete deposit",
		})
		return
	}

	// Get updated user balance
	var newBalance int
	database.DB.QueryRow("SELECT total_points FROM users WHERE id = ?", userID).Scan(&newBalance)

	log.Printf("Deposit recorded: user=%d, material=%s, weight=%.2f, points=%d", userID, req.Material, req.Weight, points)

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Message: "Deposit recorded successfully",
		Data: map[string]interface{}{
			"newBalance":    newBalance,
			"pointsEarned":  points,
			"transactionId": transactionID,
		},
	})
}
