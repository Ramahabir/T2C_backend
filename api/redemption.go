package api

import (
	"encoding/json"
	"t2cbackend/database"
	"net/http"
	"time"
)

// RedeemRequest represents a redemption request
type RedeemRequest struct {
	Points      int    `json:"points"`
	Method      string `json:"method"` // bank/cash/voucher
	AccountInfo string `json:"account_info"`
}

// getRedemptionOptions retrieves available redemption methods
func getRedemptionOptions(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromHeader(r)
	if err != nil {
		respondJSON(w, http.StatusUnauthorized, Response{
			Success: false,
			Error:   "Invalid user",
		})
		return
	}

	// Get user's current points
	var totalPoints int
	database.DB.QueryRow("SELECT total_points FROM users WHERE id = ?", userID).Scan(&totalPoints)

	options := []map[string]interface{}{
		{
			"method":         "bank",
			"name":           "Bank Transfer",
			"min_points":     1000,
			"conversion_rate": 100, // 100 points = Rp 1,000
			"description":    "Transfer to your bank account",
		},
		{
			"method":         "cash",
			"name":           "Cash Pickup",
			"min_points":     500,
			"conversion_rate": 100,
			"description":    "Pick up cash at station",
		},
		{
			"method":         "voucher",
			"name":           "Shopping Voucher",
			"min_points":     250,
			"conversion_rate": 100,
			"description":    "Redeem for shopping vouchers",
		},
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"total_points": totalPoints,
			"options":      options,
		},
	})
}

// redeemPoints processes a points redemption
func redeemPoints(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromHeader(r)
	if err != nil {
		respondJSON(w, http.StatusUnauthorized, Response{
			Success: false,
			Error:   "Invalid user",
		})
		return
	}

	var req RedeemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Invalid request body",
		})
		return
	}

	// Validate points
	if req.Points <= 0 {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Invalid points amount",
		})
		return
	}

	// Check user has enough points
	var totalPoints int
	database.DB.QueryRow("SELECT total_points FROM users WHERE id = ?", userID).Scan(&totalPoints)

	if totalPoints < req.Points {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Insufficient points",
		})
		return
	}

	// Calculate cash amount (100 points = Rp 1,000)
	amountCash := float64(req.Points) * 10.0

	// Begin transaction
	tx, err := database.DB.Begin()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to process redemption",
		})
		return
	}
	defer tx.Rollback()

	// Insert redemption record
	result, err := tx.Exec(`
		INSERT INTO redemptions (user_id, points_used, amount_cash, method, status, account_info)
		VALUES (?, ?, ?, ?, ?, ?)
	`, userID, req.Points, amountCash, req.Method, "pending", req.AccountInfo)

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to save redemption",
		})
		return
	}

	// Deduct points from user
	_, err = tx.Exec("UPDATE users SET total_points = total_points - ?, updated_at = ? WHERE id = ?",
		req.Points, time.Now(), userID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to update points",
		})
		return
	}

	// Create corresponding transaction record
	_, err = tx.Exec(`
		INSERT INTO transactions (user_id, type, amount, item_type, weight, points_earned, station_id)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, userID, "redemption", amountCash, "redemption", 0, -req.Points, 1)

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to record transaction",
		})
		return
	}

	if err = tx.Commit(); err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to complete redemption",
		})
		return
	}

	redemptionID, _ := result.LastInsertId()

	// Get updated points
	var newTotalPoints int
	database.DB.QueryRow("SELECT total_points FROM users WHERE id = ?", userID).Scan(&newTotalPoints)

	respondJSON(w, http.StatusCreated, Response{
		Success: true,
		Message: "Redemption successful",
		Data: map[string]interface{}{
			"id":             redemptionID,
			"points_redeemed": req.Points,
			"amount_cash":    amountCash,
			"method":         req.Method,
			"status":         "pending",
			"remaining_points": newTotalPoints,
		},
	})
}

// getRedemptionHistory retrieves redemption history
func getRedemptionHistory(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromHeader(r)
	if err != nil {
		respondJSON(w, http.StatusUnauthorized, Response{
			Success: false,
			Error:   "Invalid user",
		})
		return
	}

	rows, err := database.DB.Query(`
		SELECT id, points_used, amount_cash, method, status, account_info, timestamp
		FROM redemptions
		WHERE user_id = ?
		ORDER BY timestamp DESC
		LIMIT 50
	`, userID)

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to retrieve history",
		})
		return
	}
	defer rows.Close()

	var redemptions []database.Redemption
	for rows.Next() {
		var r database.Redemption
		r.UserID = userID
		err := rows.Scan(&r.ID, &r.PointsUsed, &r.AmountCash, &r.Method,
			&r.Status, &r.AccountInfo, &r.Timestamp)
		if err != nil {
			continue
		}
		redemptions = append(redemptions, r)
	}

	if redemptions == nil {
		redemptions = []database.Redemption{}
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    redemptions,
	})
}
