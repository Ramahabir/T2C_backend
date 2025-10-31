package api

import (
	"encoding/json"
	"net/http"
	"t2cbackend/database"
	"time"
)

// getUserProfile retrieves the current user's profile
func getUserProfile(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromHeader(r)
	if err != nil {
		respondJSON(w, http.StatusUnauthorized, Response{
			Success: false,
			Error:   "Invalid user",
		})
		return
	}

	var user database.User
	err = database.DB.QueryRow(`
		SELECT id, email, name, total_points, created_at, updated_at
		FROM users WHERE id = ?
	`, userID).Scan(&user.ID, &user.Email, &user.Name,
		&user.TotalPoints, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		respondJSON(w, http.StatusNotFound, Response{
			Success: false,
			Error:   "User not found",
		})
		return
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    user,
	})
}

// getUserStats retrieves user recycling statistics
func getUserStats(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromHeader(r)
	if err != nil {
		respondJSON(w, http.StatusUnauthorized, Response{
			Success: false,
			Error:   "Invalid user",
		})
		return
	}

	// Get total deposits
	var totalDeposits int
	var totalWeight float64
	var totalPointsEarned int

	database.DB.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(weight), 0), COALESCE(SUM(points_earned), 0)
		FROM transactions
		WHERE user_id = ? AND type = 'deposit'
	`, userID).Scan(&totalDeposits, &totalWeight, &totalPointsEarned)

	// Get breakdown by material
	rows, _ := database.DB.Query(`
		SELECT item_type, COUNT(*), SUM(weight), SUM(points_earned)
		FROM transactions
		WHERE user_id = ? AND type = 'deposit'
		GROUP BY item_type
	`, userID)
	defer rows.Close()

	breakdown := make(map[string]interface{})
	for rows.Next() {
		var itemType string
		var count int
		var weight float64
		var points int
		rows.Scan(&itemType, &count, &weight, &points)
		breakdown[itemType] = map[string]interface{}{
			"count":  count,
			"weight": weight,
			"points": points,
		}
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"total_deposits":      totalDeposits,
			"total_weight_kg":     totalWeight,
			"total_points_earned": totalPointsEarned,
			"breakdown":           breakdown,
		},
	})
}

// updateUserProfile updates user profile information
func updateUserProfile(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromHeader(r)
	if err != nil {
		respondJSON(w, http.StatusUnauthorized, Response{
			Success: false,
			Error:   "Invalid user",
		})
		return
	}

	var req struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Invalid request body",
		})
		return
	}

	_, err = database.DB.Exec(`
		UPDATE users SET name = ?, updated_at = ?
		WHERE id = ?
	`, req.Name, time.Now(), userID)

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to update profile",
		})
		return
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Message: "Profile updated successfully",
	})
}

// getStationStatus retrieves the current station status
func getStationStatus(w http.ResponseWriter, r *http.Request) {
	var station database.Station
	err := database.DB.QueryRow(`
		SELECT id, location, status, capacity, last_maintenance, configuration
		FROM stations WHERE id = 1
	`).Scan(&station.ID, &station.Location, &station.Status, &station.Capacity,
		&station.LastMaintenance, &station.Configuration)

	if err != nil {
		respondJSON(w, http.StatusNotFound, Response{
			Success: false,
			Error:   "Station not found",
		})
		return
	}

	// Get today's statistics
	var todayDeposits int
	var todayWeight float64
	database.DB.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(weight), 0)
		FROM transactions
		WHERE DATE(timestamp) = DATE('now') AND type = 'deposit'
	`).Scan(&todayDeposits, &todayWeight)

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"station": station,
			"today_stats": map[string]interface{}{
				"deposits":  todayDeposits,
				"weight_kg": todayWeight,
			},
		},
	})
}

// getStationConfig retrieves station configuration
func getStationConfig(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"material_rates": materialRates,
			"operating_hours": map[string]string{
				"weekday": "08:00-20:00",
				"weekend": "09:00-18:00",
			},
			"supported_materials": []string{"plastic", "glass", "metal", "paper"},
		},
	})
}
