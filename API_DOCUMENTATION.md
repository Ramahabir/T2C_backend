# T2C Backend API Documentation

## Base URL
```
http://localhost:8080
```

---

## 🔓 Public Endpoints

### Health Check
**GET** `/api/health`

Check API status.

**Response:**
```json
{
  "success": true,
  "message": "API is healthy",
  "data": {
    "status": "ok",
    "timestamp": "2025-10-31T10:00:00Z"
  }
}
```

---

## 🔐 Authentication APIs

### Register
**POST** `/api/auth/register`

Register a new user account.

**Request:**
```json
{
  "email": "user@example.com",
  "password": "password123",
  "full_name": "John Doe"
}
```

**Response:**
```json
{
  "success": true,
  "message": "Registration successful",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "user": {
      "id": 3,
      "email": "user@example.com",
      "full_name": "John Doe",
      "total_points": 0
    }
  }
}
```

### Login
**POST** `/api/auth/login`

Login with email and password.

**Request:**
```json
{
  "email": "user@example.com",
  "password": "password123"
}
```

**Response:**
```json
{
  "success": true,
  "message": "Login successful",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "user": {
      "id": 1,
      "email": "user@example.com",
      "full_name": "John Doe",
      "total_points": 1500
    }
  }
}
```

### Logout
**POST** `/api/auth/logout`

Logout and invalidate session.

**Request:**
```json
{
  "token": "session_token_here"
}
```

**Response:**
```json
{
  "success": true,
  "message": "Logged out successfully"
}
```

---

## 🎯 Session Management APIs (QR Flow)

### Request Session
**POST** `/api/request-session`

Creates a new station session and generates a QR code.

**Request (optional):**
```json
{
  "station_id": "station_001"
}
```

**Response:**
```json
{
  "success": true,
  "message": "Session token generated",
  "data": {
    "sessionToken": "550e8400-e29b-41d4-a716-446655440000",
    "qrCode": "data:image/png;base64,iVBORw0KG...",
    "expiresAt": "2025-10-31T12:05:00Z",
    "status": "pending"
  }
}
```

### Check Session
**POST** `/api/check-session`

Station polls to check if user has connected.

**Request:**
```json
{
  "sessionToken": "550e8400-e29b-41d4-a716-446655440000"
}
```

**Response (pending):**
```json
{
  "success": true,
  "data": {
    "status": "pending"
  }
}
```

**Response (connected):**
```json
{
  "success": true,
  "data": {
    "status": "connected",
    "authToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "userId": 1,
    "userName": "John Doe",
    "userBalance": 1500
  }
}
```

### Connect Session
**POST** `/api/connect-session`

Mobile app connects authenticated user to station session.

**Request:**
```json
{
  "sessionToken": "550e8400-e29b-41d4-a716-446655440000",
  "authToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**Response:**
```json
{
  "success": true,
  "message": "User connected to station session"
}
```

### End Session
**POST** `/api/end-session`

Ends the current recycling session.

**Request:**
```json
{
  "sessionToken": "550e8400-e29b-41d4-a716-446655440000"
}
```

**Response:**
```json
{
  "success": true,
  "message": "Session ended"
}
```

---

## 🔒 Protected APIs (Require Authentication)

**All protected endpoints require:**
```
Authorization: Bearer <jwt_token>
```

### User Management

#### Get User Profile
**GET** `/api/user/profile`

Get current user's profile information.

**Response:**
```json
{
  "success": true,
  "data": {
    "id": 1,
    "email": "user@example.com",
    "full_name": "John Doe",
    "total_points": 1500,
    "created_at": "2025-10-01T10:00:00Z",
    "updated_at": "2025-10-31T10:00:00Z"
  }
}
```

#### Get User Stats
**GET** `/api/user/stats`

Get user's recycling statistics.

**Response:**
```json
{
  "success": true,
  "data": {
    "total_deposits": 25,
    "total_weight_kg": 37.5,
    "total_points_earned": 1500,
    "breakdown": {
      "plastic": {
        "count": 10,
        "weight": 15.0,
        "points": 150
      },
      "metal": {
        "count": 5,
        "weight": 7.5,
        "points": 112
      }
    }
  }
}
```

#### Update User Profile
**PUT** `/api/user/profile`

Update user profile information.

**Request:**
```json
{
  "full_name": "John Smith"
}
```

**Response:**
```json
{
  "success": true,
  "message": "Profile updated successfully"
}
```

---

### Transactions

#### Get Transactions
**GET** `/api/transactions?limit=50&offset=0`

Get transaction history with pagination.

**Response:**
```json
{
  "success": true,
  "data": [
    {
      "id": 1,
      "user_id": 1,
      "type": "deposit",
      "amount": 0,
      "item_type": "plastic",
      "weight": 1.5,
      "points_earned": 15,
      "station_id": 1,
      "timestamp": "2025-10-31T10:00:00Z"
    }
  ]
}
```

#### Create Transaction
**POST** `/api/transactions`

Create a new transaction (manual entry).

**Request:**
```json
{
  "item_type": "plastic",
  "weight": 1.5
}
```

**Response:**
```json
{
  "success": true,
  "message": "Transaction successful",
  "data": {
    "id": 123,
    "points_earned": 15,
    "total_points": 1515
  }
}
```

#### Get Transaction Detail
**GET** `/api/transactions/{id}`

Get specific transaction details.

**Response:**
```json
{
  "success": true,
  "data": {
    "id": 1,
    "user_id": 1,
    "type": "deposit",
    "amount": 0,
    "item_type": "plastic",
    "weight": 1.5,
    "points_earned": 15,
    "station_id": 1,
    "timestamp": "2025-10-31T10:00:00Z"
  }
}
```

---

### Deposit (Session-Based)

#### Process Deposit
**POST** `/api/deposit`

Process item deposit during active session.

**Request:**
```json
{
  "material": "plastic",
  "weight": 1.5,
  "sessionToken": "550e8400-e29b-41d4-a716-446655440000"
}
```

**Response:**
```json
{
  "success": true,
  "message": "Deposit recorded successfully",
  "data": {
    "newBalance": 1515,
    "pointsEarned": 15,
    "transactionId": 123
  }
}
```

---

### Redemptions

#### Get Redemption Options
**GET** `/api/redemption/options`

Get available redemption options and user's total points.

**Response:**
```json
{
  "success": true,
  "data": {
    "total_points": 1500,
    "options": [
      {
        "method": "bank",
        "name": "Bank Transfer",
        "min_points": 1000,
        "conversion_rate": 100,
        "description": "Transfer to your bank account"
      }
    ]
  }
}
```

#### Redeem Points
**POST** `/api/redemption/redeem`

Redeem points for cash/bank/voucher.

**Request:**
```json
{
  "points": 1000,
  "method": "bank",
  "account_info": "1234567890"
}
```

**Response:**
```json
{
  "success": true,
  "message": "Redemption successful",
  "data": {
    "id": 1,
    "points_redeemed": 1000,
    "amount_cash": 10000,
    "method": "bank",
    "status": "pending",
    "remaining_points": 500
  }
}
```

#### Get Redemption History
**GET** `/api/redemption/history`

Get redemption history.

**Response:**
```json
{
  "success": true,
  "data": [
    {
      "id": 1,
      "user_id": 1,
      "points_used": 1000,
      "amount_cash": 10000,
      "method": "bank",
      "status": "pending",
      "account_info": "1234567890",
      "timestamp": "2025-10-31T10:00:00Z"
    }
  ]
}
```

---

### Station Management

#### Get Station Status
**GET** `/api/station/status`

Get recycling station status and today's stats.

**Response:**
```json
{
  "success": true,
  "data": {
    "station": {
      "id": 1,
      "location": "Main Station",
      "status": "active",
      "capacity": 100,
      "last_maintenance": "2025-10-01T10:00:00Z",
      "configuration": null
    },
    "today_stats": {
      "deposits": 15,
      "weight_kg": 22.5
    }
  }
}
```

#### Process Station Deposit
**POST** `/api/station/deposit`

Process deposit at station (legacy endpoint).

**Request:**
```json
{
  "item_type": "plastic",
  "weight": 1.5
}
```

**Response:**
```json
{
  "success": true,
  "message": "Deposit processed successfully",
  "data": {
    "id": 123,
    "item_type": "plastic",
    "weight": 1.5,
    "points_earned": 15,
    "total_points": 1515,
    "timestamp": "2025-10-31T10:00:00Z"
  }
}
```

#### Get Station Config
**GET** `/api/station/config`

Get station configuration (material rates, operating hours).

**Response:**
```json
{
  "success": true,
  "data": {
    "material_rates": {
      "plastic": 10,
      "glass": 8,
      "metal": 15,
      "paper": 5
    },
    "operating_hours": {
      "weekday": "08:00-20:00",
      "weekend": "09:00-18:00"
    },
    "supported_materials": ["plastic", "glass", "metal", "paper"]
  }
}
```

---

## 📊 Material Points Calculation

| Material | Points per kg |
|----------|---------------|
| Plastic  | 10            |
| Glass    | 8             |
| Metal    | 15            |
| Paper    | 5             |

---

## 💰 Redemption Info

- **Conversion Rate:** 100 points = Rp 1,000
- **Methods:** 
  - Bank Transfer (min: 1000 points)
  - Cash Pickup (min: 500 points)
  - Shopping Voucher (min: 250 points)

---

## 👥 Demo Users

1. **Email:** `dummy@trash2cash.com` | **Password:** `dummy123` | **Points:** 1000
2. **Email:** `demo@trash2cash.com` | **Password:** `demo123` | **Points:** 2500

---

## 🔄 QR Session Flow

1. **Station** calls `POST /api/request-session` → displays QR code
2. **User** scans QR code with mobile app → extracts session token
3. **Mobile App** calls `POST /api/connect-session` with session token + JWT
4. **Station** polls `POST /api/check-session` → gets user details when connected
5. **User** deposits items → `POST /api/deposit` with session token
6. **Station** calls `POST /api/end-session` when done

---

## ⚠️ Error Responses

All error responses follow this format:

```json
{
  "success": false,
  "error": "Error message description"
}
```

Common HTTP status codes:
- `400` - Bad Request (invalid input)
- `401` - Unauthorized (invalid/missing token)
- `403` - Forbidden (insufficient permissions)
- `404` - Not Found
- `409` - Conflict (duplicate/already exists)
- `500` - Internal Server Error

---

## 🔧 WebSocket

**GET** `/ws`

WebSocket connection for real-time updates (echo server for now).

---

## 📝 Notes

- All timestamps are in RFC3339 format
- Session tokens expire after 5 minutes
- JWT tokens expire after 24 hours
- Total points stored as integers
- Weights stored as floating-point numbers (kg)
