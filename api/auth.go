package api

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"t2cbackend/database"
	"net/http"
	"time"

	"github.com/google/uuid"
	qrcode "github.com/skip2/go-qrcode"
)

// Register request
type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
}

// Login request
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// QR Login request
type QRLoginRequest struct {
	Token string `json:"token"`
}

// Verify token request
type VerifyTokenRequest struct {
	QRToken  string `json:"qr_token"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// login authenticates a user with email and password
func login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Invalid request body",
		})
		return
	}

	// Validate input
	if req.Email == "" || req.Password == "" {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Email and password are required",
		})
		return
	}

	// Find user by email
	var user database.User
	var hashedPassword string

	err := database.DB.QueryRow(
		"SELECT id, email, password, full_name, total_points FROM users WHERE email = ?",
		req.Email,
	).Scan(&user.ID, &user.Email, &hashedPassword, &user.FullName, &user.TotalPoints)

	if err != nil {
		log.Printf("Login failed: user not found for email %s, error: %v", req.Email, err)
		respondJSON(w, http.StatusUnauthorized, Response{
			Success: false,
			Error:   "Invalid email or password",
		})
		return
	}

	log.Printf("Login attempt: email=%s, user_id=%d", req.Email, user.ID)

	// Verify password
	if !checkPasswordHash(req.Password, hashedPassword) {
		log.Printf("Login failed: password mismatch for email %s", req.Email)
		respondJSON(w, http.StatusUnauthorized, Response{
			Success: false,
			Error:   "Invalid email or password",
		})
		return
	}

	log.Printf("Login successful: email=%s, user_id=%d", req.Email, user.ID)

	// Generate JWT token
	jwtToken, err := generateJWT(user.ID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to generate authentication token",
		})
		return
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Message: "Login successful",
		Data: map[string]interface{}{
			"token": jwtToken,
			"user": map[string]interface{}{
				"id":           user.ID,
				"email":        user.Email,
				"full_name":    user.FullName,
				"total_points": user.TotalPoints,
			},
		},
	})
}

// register creates a new user account
func register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Invalid request body",
		})
		return
	}

	// Validate input
	if req.Email == "" || req.Password == "" || req.FullName == "" {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Email, password, and full name are required",
		})
		return
	}

	// Hash password
	hashedPassword, err := hashPassword(req.Password)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to process password",
		})
		return
	}

	// Insert user
	result, err := database.DB.Exec(
		"INSERT INTO users (email, password, full_name, total_points) VALUES (?, ?, ?, ?)",
		req.Email, hashedPassword, req.FullName, 0,
	)
	if err != nil {
		respondJSON(w, http.StatusConflict, Response{
			Success: false,
			Error:   "Email already exists",
		})
		return
	}

	userID, _ := result.LastInsertId()

	respondJSON(w, http.StatusCreated, Response{
		Success: true,
		Message: "Registration successful",
		Data: map[string]interface{}{
			"id":         userID,
			"email":      req.Email,
			"full_name":  req.FullName,
		},
	})
}

// generateQRLogin generates a new QR code for login
func generateQRLogin(w http.ResponseWriter, r *http.Request) {
	// Generate unique session token
	token := uuid.New().String()
	qrToken := uuid.New().String()

	log.Printf("Generating QR login: token=%s, qrToken=%s", token, qrToken)

	// Set expiration time (5 minutes from now)
	expiresAt := time.Now().Add(5 * time.Minute)

	// Insert session into database
	_, err := database.DB.Exec(
		"INSERT INTO sessions (token, qr_token, expires_at) VALUES (?, ?, ?)",
		token, qrToken, expiresAt,
	)
	if err != nil {
		log.Printf("Failed to insert session into database: %v", err)
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to generate QR code session",
		})
		return
	}

	log.Printf("Session inserted successfully")

	// Generate QR code image
	qrBytes, err := qrcode.Encode(qrToken, qrcode.Medium, 256)
	if err != nil {
		log.Printf("Failed to encode QR code: %v", err)
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to generate QR code image",
		})
		return
	}

	log.Printf("QR code generated successfully")

	// Convert to base64
	qrBase64 := base64.StdEncoding.EncodeToString(qrBytes)

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Message: "QR code generated",
		Data: map[string]interface{}{
			"token":      token,
			"qr_token":   qrToken,
			"qr_code":    "data:image/png;base64," + qrBase64,
			"expires_at": expiresAt.Format(time.RFC3339),
		},
	})
}

// verifyToken verifies and authenticates a QR token
func verifyToken(w http.ResponseWriter, r *http.Request) {
	var req VerifyTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Invalid request body",
		})
		return
	}

	// Verify session exists
	var sessionID int
	var token string
	var userID *int
	var expiresAt time.Time

	err := database.DB.QueryRow(
		"SELECT id, token, user_id, expires_at FROM sessions WHERE qr_token = ?",
		req.QRToken,
	).Scan(&sessionID, &token, &userID, &expiresAt)

	if err != nil {
		respondJSON(w, http.StatusNotFound, Response{
			Success: false,
			Error:   "Invalid QR token",
		})
		return
	}

	// Check if expired
	if time.Now().After(expiresAt) {
		respondJSON(w, http.StatusUnauthorized, Response{
			Success: false,
			Error:   "QR token has expired",
		})
		return
	}

	// Authenticate user
	var user database.User
	var hashedPassword string

	err = database.DB.QueryRow(
		"SELECT id, email, password, full_name, total_points FROM users WHERE email = ?",
		req.Email,
	).Scan(&user.ID, &user.Email, &hashedPassword, &user.FullName, &user.TotalPoints)

	if err != nil {
		respondJSON(w, http.StatusUnauthorized, Response{
			Success: false,
			Error:   "Invalid credentials",
		})
		return
	}

	// Check password
	if !checkPasswordHash(req.Password, hashedPassword) {
		respondJSON(w, http.StatusUnauthorized, Response{
			Success: false,
			Error:   "Invalid credentials",
		})
		return
	}

	// Update session with user ID
	_, err = database.DB.Exec(
		"UPDATE sessions SET user_id = ? WHERE id = ?",
		user.ID, sessionID,
	)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to update session",
		})
		return
	}

	// Generate JWT
	jwtToken, err := generateJWT(user.ID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to generate token",
		})
		return
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Message: "Authentication successful",
		Data: map[string]interface{}{
			"token":        jwtToken,
			"session_token": token,
			"user": map[string]interface{}{
				"id":           user.ID,
				"email":        user.Email,
				"full_name":    user.FullName,
				"total_points": user.TotalPoints,
			},
		},
	})
}

// logout invalidates the current session
func logout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Invalid request body",
		})
		return
	}

	// Delete session
	_, err := database.DB.Exec("DELETE FROM sessions WHERE token = ?", req.Token)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to logout",
		})
		return
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Message: "Logged out successfully",
	})
}
