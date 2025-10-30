package database

import (
	"database/sql"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

var DB *sql.DB

// User represents a user in the system
type User struct {
	ID          int       `json:"id"`
	Email       string    `json:"email"`
	Password    string    `json:"-"`
	FullName    string    `json:"full_name"`
	TotalPoints int       `json:"total_points"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Transaction represents a deposit or redemption transaction
type Transaction struct {
	ID           int       `json:"id"`
	UserID       int       `json:"user_id"`
	Type         string    `json:"type"` // deposit/redemption
	Amount       float64   `json:"amount"`
	ItemType     string    `json:"item_type"`
	Weight       float64   `json:"weight"`
	PointsEarned int       `json:"points_earned"`
	StationID    int       `json:"station_id"`
	Timestamp    time.Time `json:"timestamp"`
}

// Redemption represents a points redemption
type Redemption struct {
	ID          int       `json:"id"`
	UserID      int       `json:"user_id"`
	PointsUsed  int       `json:"points_used"`
	AmountCash  float64   `json:"amount_cash"`
	Method      string    `json:"method"` // bank/cash/voucher
	Status      string    `json:"status"`
	AccountInfo string    `json:"account_info"`
	Timestamp   time.Time `json:"timestamp"`
}

// Session represents a user session
type Session struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	Token     string    `json:"token"`
	QRToken   string    `json:"qr_token"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// Station represents a recycling station
type Station struct {
	ID              int       `json:"id"`
	Location        string    `json:"location"`
	Status          string    `json:"status"`
	Capacity        int       `json:"capacity"`
	LastMaintenance time.Time `json:"last_maintenance"`
	Configuration   string    `json:"configuration"`
}

// InitDB initializes the database connection and creates tables
func InitDB() error {
	var err error
	DB, err = sql.Open("sqlite3", "./trash2cash.db")
	if err != nil {
		return err
	}

	// Create users table
	createUsersTable := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		email TEXT UNIQUE NOT NULL,
		password TEXT NOT NULL,
		full_name TEXT NOT NULL,
		total_points INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	_, err = DB.Exec(createUsersTable)
	if err != nil {
		return err
	}

	// Create transactions table
	createTransactionsTable := `
	CREATE TABLE IF NOT EXISTS transactions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		type TEXT NOT NULL,
		amount REAL DEFAULT 0,
		item_type TEXT NOT NULL,
		weight REAL NOT NULL,
		points_earned INTEGER NOT NULL,
		station_id INTEGER DEFAULT 1,
		session_token TEXT,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id)
	);`

	_, err = DB.Exec(createTransactionsTable)
	if err != nil {
		return err
	}

	// Create redemptions table
	createRedemptionsTable := `
	CREATE TABLE IF NOT EXISTS redemptions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		points_used INTEGER NOT NULL,
		amount_cash REAL NOT NULL,
		method TEXT NOT NULL,
		status TEXT DEFAULT 'pending',
		account_info TEXT,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id)
	);`

	_, err = DB.Exec(createRedemptionsTable)
	if err != nil {
		return err
	}

	// Create sessions table
	createSessionsTable := `
	CREATE TABLE IF NOT EXISTS sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER,
		token TEXT UNIQUE NOT NULL,
		qr_token TEXT UNIQUE,
		expires_at DATETIME NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id)
	);`

	_, err = DB.Exec(createSessionsTable)
	if err != nil {
		return err
	}

	// Create login_sessions table for QR code login
	createLoginSessionsTable := `
	CREATE TABLE IF NOT EXISTS login_sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		token TEXT UNIQUE NOT NULL,
		status TEXT DEFAULT 'pending',
		user_id INTEGER,
		expires_at DATETIME NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id)
	);`

	_, err = DB.Exec(createLoginSessionsTable)
	if err != nil {
		return err
	}

	// Create stations table
	createStationsTable := `
	CREATE TABLE IF NOT EXISTS stations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		location TEXT NOT NULL,
		status TEXT DEFAULT 'active',
		capacity INTEGER DEFAULT 100,
		last_maintenance DATETIME DEFAULT CURRENT_TIMESTAMP,
		configuration TEXT
	);`

	_, err = DB.Exec(createStationsTable)
	if err != nil {
		return err
	}

	// Create station_sessions table for QR session management
	createStationSessionsTable := `
	CREATE TABLE IF NOT EXISTS station_sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_token TEXT UNIQUE NOT NULL,
		station_id TEXT DEFAULT 'default',
		user_id INTEGER,
		status TEXT DEFAULT 'pending',
		auth_token TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		expires_at DATETIME NOT NULL,
		ended_at DATETIME,
		FOREIGN KEY (user_id) REFERENCES users(id)
	);`

	_, err = DB.Exec(createStationSessionsTable)
	if err != nil {
		return err
	}

	// Create index for better query performance
	DB.Exec(`CREATE INDEX IF NOT EXISTS idx_session_token ON station_sessions(session_token)`)
	DB.Exec(`CREATE INDEX IF NOT EXISTS idx_session_status ON station_sessions(status)`)
	DB.Exec(`CREATE INDEX IF NOT EXISTS idx_session_expires ON station_sessions(expires_at)`)

	// Insert default station if not exists
	DB.Exec(`INSERT OR IGNORE INTO stations (id, location, status, capacity) VALUES (1, 'Main Station', 'active', 100)`)

	// Insert dummy user if not exists
	// Hash the password "dummy123"
	dummyPasswordHash, err := bcrypt.GenerateFromPassword([]byte("dummy123"), bcrypt.DefaultCost)
	if err == nil {
		DB.Exec(`INSERT OR IGNORE INTO users (id, email, password, full_name, total_points) 
			VALUES (1, 'dummy@trash2cash.com', ?, 'Dummy User', 1000)`, string(dummyPasswordHash))
		log.Println("Dummy user created: dummy@trash2cash.com / dummy123")
	}

	// Insert demo user if not exists
	// Hash the password "demo123"
	demoPasswordHash, err := bcrypt.GenerateFromPassword([]byte("demo123"), bcrypt.DefaultCost)
	if err == nil {
		DB.Exec(`INSERT OR IGNORE INTO users (id, email, password, full_name, total_points) 
			VALUES (2, 'demo@trash2cash.com', ?, 'Demo User', 2500)`, string(demoPasswordHash))
		log.Println("Demo user created: demo@trash2cash.com / demo123")
	}

	log.Println("Database initialized successfully")
	return nil
}

// CloseDB closes the database connection
func CloseDB() {
	if DB != nil {
		DB.Close()
	}
}
