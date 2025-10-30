package api

import (
	"encoding/json"
	"t2cbackend/database"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

// Material rates for points calculation
var materialRates = map[string]int{
	"plastic": 10, // 10 points per kg
	"glass":   8,
	"metal":   15,
	"paper":   5,
}

// CalculatePoints calculates points based on item type and weight
func CalculatePoints(itemType string, weight float64) int {
	rate, exists := materialRates[itemType]
	if !exists {
		return 0
	}
	return int(weight * float64(rate))
}

// DepositRequest represents a deposit request
type DepositRequest struct {
	ItemType string  `json:"item_type"`
	Weight   float64 `json:"weight"`
}

// getTransactions retrieves transaction history
func getTransactions(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromHeader(r)
	if err != nil {
		respondJSON(w, http.StatusUnauthorized, Response{
			Success: false,
			Error:   "Invalid user",
		})
		return
	}

	// Get query parameters
	limit := 50
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		limit, _ = strconv.Atoi(l)
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		offset, _ = strconv.Atoi(o)
	}

	rows, err := database.DB.Query(`
		SELECT id, user_id, type, amount, item_type, weight, points_earned, station_id, timestamp
		FROM transactions
		WHERE user_id = ?
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?
	`, userID, limit, offset)

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to retrieve transactions",
		})
		return
	}
	defer rows.Close()

	var transactions []database.Transaction
	for rows.Next() {
		var tx database.Transaction
		err := rows.Scan(&tx.ID, &tx.UserID, &tx.Type, &tx.Amount, &tx.ItemType,
			&tx.Weight, &tx.PointsEarned, &tx.StationID, &tx.Timestamp)
		if err != nil {
			continue
		}
		transactions = append(transactions, tx)
	}

	if transactions == nil {
		transactions = []database.Transaction{}
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    transactions,
	})
}

// createTransaction creates a new transaction (manual entry)
func createTransaction(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromHeader(r)
	if err != nil {
		respondJSON(w, http.StatusUnauthorized, Response{
			Success: false,
			Error:   "Invalid user",
		})
		return
	}

	var req DepositRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Invalid request body",
		})
		return
	}

	// Calculate points
	points := CalculatePoints(req.ItemType, req.Weight)
	if points == 0 {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Invalid item type",
		})
		return
	}

	// Begin transaction
	tx, err := database.DB.Begin()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to process transaction",
		})
		return
	}
	defer tx.Rollback()

	// Insert transaction
	result, err := tx.Exec(`
		INSERT INTO transactions (user_id, type, item_type, weight, points_earned, station_id)
		VALUES (?, ?, ?, ?, ?, ?)
	`, userID, "deposit", req.ItemType, req.Weight, points, 1)

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to save transaction",
		})
		return
	}

	// Update user points
	_, err = tx.Exec("UPDATE users SET total_points = total_points + ?, updated_at = ? WHERE id = ?",
		points, time.Now(), userID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to update points",
		})
		return
	}

	if err = tx.Commit(); err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to complete transaction",
		})
		return
	}

	txID, _ := result.LastInsertId()

	// Get updated points
	var totalPoints int
	database.DB.QueryRow("SELECT total_points FROM users WHERE id = ?", userID).Scan(&totalPoints)

	respondJSON(w, http.StatusCreated, Response{
		Success: true,
		Message: "Transaction successful",
		Data: map[string]interface{}{
			"id":            txID,
			"points_earned": points,
			"total_points":  totalPoints,
		},
	})
}

// getTransactionDetail retrieves a specific transaction
func getTransactionDetail(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromHeader(r)
	if err != nil {
		respondJSON(w, http.StatusUnauthorized, Response{
			Success: false,
			Error:   "Invalid user",
		})
		return
	}

	txID := chi.URLParam(r, "id")

	var tx database.Transaction
	err = database.DB.QueryRow(`
		SELECT id, user_id, type, amount, item_type, weight, points_earned, station_id, timestamp
		FROM transactions
		WHERE id = ? AND user_id = ?
	`, txID, userID).Scan(&tx.ID, &tx.UserID, &tx.Type, &tx.Amount, &tx.ItemType,
		&tx.Weight, &tx.PointsEarned, &tx.StationID, &tx.Timestamp)

	if err != nil {
		respondJSON(w, http.StatusNotFound, Response{
			Success: false,
			Error:   "Transaction not found",
		})
		return
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    tx,
	})
}

// processDeposit processes an item deposit at the station
func processDeposit(w http.ResponseWriter, r *http.Request) {
	// This endpoint would typically be called by station hardware
	// For now, it's similar to createTransaction but can include station-specific logic

	userID, err := getUserIDFromHeader(r)
	if err != nil {
		respondJSON(w, http.StatusUnauthorized, Response{
			Success: false,
			Error:   "Invalid user",
		})
		return
	}

	var req DepositRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Invalid request body",
		})
		return
	}

	// Validate and calculate points
	points := CalculatePoints(req.ItemType, req.Weight)
	if points == 0 {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Invalid item type",
		})
		return
	}

	// Begin transaction
	tx, err := database.DB.Begin()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to process deposit",
		})
		return
	}
	defer tx.Rollback()

	// Insert transaction
	result, err := tx.Exec(`
		INSERT INTO transactions (user_id, type, item_type, weight, points_earned, station_id)
		VALUES (?, ?, ?, ?, ?, ?)
	`, userID, "deposit", req.ItemType, req.Weight, points, 1)

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to save deposit",
		})
		return
	}

	// Update user points
	_, err = tx.Exec("UPDATE users SET total_points = total_points + ?, updated_at = ? WHERE id = ?",
		points, time.Now(), userID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to update points",
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

	depositID, _ := result.LastInsertId()

	// Get updated points
	var totalPoints int
	database.DB.QueryRow("SELECT total_points FROM users WHERE id = ?", userID).Scan(&totalPoints)

	respondJSON(w, http.StatusCreated, Response{
		Success: true,
		Message: "Deposit processed successfully",
		Data: map[string]interface{}{
			"id":            depositID,
			"item_type":     req.ItemType,
			"weight":        req.Weight,
			"points_earned": points,
			"total_points":  totalPoints,
			"timestamp":     time.Now(),
		},
	})
}
