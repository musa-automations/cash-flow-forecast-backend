# Cash Flow Forecast Backend

A robust and scalable REST API built with Go and Gin for managing personal cash flow forecasting. This backend provides user authentication, cash entry management, and financial data analysis capabilities.

## Features

- **User Authentication** — Secure signup, login, and JWT-based session management
- **Cash Entry Management** — Create, read, update, and delete cash entries (inflows/outflows)
- **Forecast Management** — Create multiple forecasts per user and manage their entries independently
- **Bulk Operations** — Import multiple cash entries at once
- **CSV/XLSX Import** — Upload spreadsheet files into a specific forecast
- **UUID-based IDs** — Secure, collision-resistant identifiers
- **Neon PostgreSQL** — Persistent cloud database storage
- **CORS Support** — Cross-origin request handling
- **Docker Ready** — Containerized deployment with multi-stage builds
- **Structured Logging** — Request/response tracking and error diagnostics

## Tech Stack

- **Language:** Go 1.21+
- **Framework:** Gin Web Framework
- **Database:** PostgreSQL (Neon) with GORM ORM
- **Authentication:** JWT (github.com/golang-jwt/jwt)
- **ID Generation:** UUID (github.com/google/uuid)
- **Spreadsheet Import:** Excelize (github.com/xuri/excelize/v2)
- **Environment:** godotenv

## Prerequisites

- Go 1.21 or higher
- A Neon PostgreSQL database URL
- Docker & Docker Compose (optional)

## Installation

### Clone the Repository

```bash
git clone github.com/waltertaya
cd cash-flow-forecast-backend
```

### Install Dependencies

```bash
make deps
```

Or manually:

```bash
go mod download
go mod tidy
```

### Environment Setup

Create a `.env` file in the project root:

```env
JWT_SECRET=your-secret-key-here
PORT=8080
DATABASE_URL=postgresql://<user>:<password>@<host>/<database>?sslmode=require
```

## Running the Project

### Local Development

```bash
make run
```

Or:

```bash
go run main.go
```

The server will start at `http://localhost:8080`

### Using Docker

Build and run with Docker:

```bash
make docker-run
```

Or manually:

```bash
docker build -t cash-flow-forecast:latest .
docker run -p 8080:8080 --env-file .env cash-flow-forecast:latest
```

## API Routes

### Base URL

```
http://localhost:8080/api/v1
```

### Authentication Routes

#### Sign Up

- **Endpoint:** `POST /auth/signup`
- **Description:** Register a new user
- **Request Body:**
  ```json
  {
    "email": "user@example.com",
    "password": "secure_password"
  }
  ```
- **Response:** `201 Created`
  ```json
  {
    "message": "User created successfully"
  }
  ```
- **Validation:**
  - Email: required, valid email format
  - Password: required, minimum 6 characters

#### Login

- **Endpoint:** `POST /auth/login`
- **Description:** Authenticate and receive auth token
- **Request Body:**
  ```json
  {
    "email": "user@example.com",
    "password": "secure_password"
  }
  ```
- **Response:** `200 OK`
  ```json
  {
    "message": "Logged in successfully"
  }
  ```
- **Sets:** HTTP-only cookie `auth_token` (valid 24 hours)

#### Logout

- **Endpoint:** `POST /auth/logout`
- **Description:** Clear authentication session
- **Auth:** Required
- **Response:** `200 OK`
  ```json
  {
    "message": "Logged out successfully"
  }
  ```

#### Get Current User

- **Endpoint:** `GET /auth/me`
- **Description:** Retrieve authenticated user info
- **Auth:** Required (JWT token)
- **Response:** `200 OK`
  ```json
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "user@example.com"
  }
  ```

### Forecast Routes

All forecast routes require authentication (JWT token in `auth_token` cookie).

#### Create Forecast

- **Endpoint:** `POST /forecasts`
- **Description:** Create a new forecast for the authenticated user
- **Auth:** Required
- **Request Body:**
  ```json
  {
    "name": "Q2 2026 Forecast",
    "starting_cash": 1000,
    "entries": [
      {
        "type": "inflow",
        "amount": 1500,
        "category": "salary",
        "description": "Monthly salary",
        "date": "2026-05-15"
      }
    ]
  }
  ```
- **Response:** `201 Created`
- **Notes:**
  - `entries` is optional
  - Any entries included here are automatically linked to the new forecast

#### List Forecasts

- **Endpoint:** `GET /forecasts`
- **Description:** Return all forecasts for the authenticated user, each with generated 13-week data
- **Auth:** Required
- **Response:** `200 OK`
  ```json
  [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "user_id": "660e8400-e29b-41d4-a716-446655440000",
      "name": "Q2 2026 Forecast",
      "starting_cash": 1000,
      "weeks": [
        {
          "week": 1,
          "opening": 1000,
          "inflow": 1500,
          "outflow": 500,
          "closing": 2000,
          "warning": false
        }
      ],
      "created_at": 1704067200,
      "updated_at": 1704067200
    }
  ]
  ```

#### View Forecast

- **Endpoint:** `GET /forecasts/:id`
- **Description:** Return one forecast with its generated 13-week data
- **Auth:** Required
- **URL Parameters:**
  - `id`: UUID of the forecast
- **Response:** `200 OK`
  ```json
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "user_id": "660e8400-e29b-41d4-a716-446655440000",
    "name": "Q2 2026 Forecast",
    "starting_cash": 1000,
    "weeks": [
      {
        "week": 1,
        "opening": 1000,
        "inflow": 1500,
        "outflow": 500,
        "closing": 2000,
        "warning": false
      }
    ],
    "created_at": 1704067200,
    "updated_at": 1704067200
  }
  ```

#### Update Forecast

- **Endpoint:** `PUT /forecasts/:id`
- **Description:** Update forecast name and/or starting cash
- **Auth:** Required
- **URL Parameters:**
  - `id`: UUID of the forecast
- **Request Body:**
  ```json
  {
    "name": "Updated Forecast Name",
    "starting_cash": 2500
  }
  ```
- **Response:** `200 OK`

#### Delete Forecast

- **Endpoint:** `DELETE /forecasts/:id`
- **Description:** Delete a forecast and all entries attached to it
- **Auth:** Required
- **URL Parameters:**
  - `id`: UUID of the forecast
- **Response:** `200 OK`
  ```json
  {
    "message": "Forecast deleted successfully"
  }
  ```

#### Import Forecast Entries from CSV/XLSX

- **Endpoint:** `POST /forecasts/:id/import`
- **Description:** Upload a CSV or Excel file and create entries under a specific forecast
- **Auth:** Required
- **Content Type:** `multipart/form-data`
- **Form Fields:**
  - `file`: CSV, XLSX, or XLSM file
- **Required Columns:** `type`, `amount`, `date`
- **Optional Columns:** `category`, `description`
- **Accepted Date Formats:**
  - `YYYY-MM-DD`
  - `YYYY/MM/DD`
  - `DD/MM/YYYY`
  - `MM/DD/YYYY`
  - Common long-date formats and Excel serial dates
- **Response:** `201 Created`
  ```json
  {
    "message": "entries imported successfully",
    "forecast_id": "550e8400-e29b-41d4-a716-446655440000",
    "imported_count": 5,
    "entries": []
  }
  ```

### Cash Entry Routes

All entry routes require authentication (JWT token in `auth_token` cookie).

#### Get All Entries

- **Endpoint:** `GET /entries`
- **Description:** Retrieve all cash entries for a specific forecast owned by the authenticated user
- **Auth:** Required
- **Query Parameters:**
  - `forecast_id`: UUID of the forecast
- **Response:** `200 OK`
  ```json
  [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "user_id": "660e8400-e29b-41d4-a716-446655440000",
      "forecast_id": "770e8400-e29b-41d4-a716-446655440000",
      "type": "inflow",
      "amount": 1500.00,
      "category": "salary",
      "description": "Monthly salary",
      "date": "2024-05-01",
      "created_at": 1704067200
    }
  ]
  ```

#### Create Cash Entry

- **Endpoint:** `POST /entries`
- **Description:** Add a single cash entry to a specific forecast
- **Auth:** Required
- **Request Body:**
  ```json
  {
    "forecast_id": "770e8400-e29b-41d4-a716-446655440000",
    "type": "inflow",
    "amount": 1500.00,
    "category": "salary",
    "description": "Monthly salary",
    "date": "2024-05-01"
  }
  ```
- **Response:** `201 Created`
- **Validation:**
  - forecast_id: required, UUID of an existing forecast owned by the user
  - type: required, must be "inflow" or "outflow"
  - amount: required, numeric
  - category: optional
  - description: optional
  - date: required, date format

#### Create Multiple Entries

- **Endpoint:** `POST /entries/bulk`
- **Description:** Import multiple cash entries into a specific forecast
- **Auth:** Required
- **Request Body:**
  ```json
  {
    "forecast_id": "770e8400-e29b-41d4-a716-446655440000",
    "entries": [
      {
        "type": "inflow",
        "amount": 1500.00,
        "category": "salary",
        "description": "Monthly salary",
        "date": "2024-05-01"
      },
      {
        "type": "outflow",
        "amount": 200.00,
        "category": "utilities",
        "description": "Electric bill",
        "date": "2024-05-02"
      }
    ]
  }
  ```
- **Response:** `201 Created` (array of created entries)

#### Update Cash Entry

- **Endpoint:** `PUT /entries/:id`
- **Description:** Update a specific cash entry
- **Auth:** Required
- **URL Parameters:**
  - `id`: UUID of the cash entry
- **Request Body:** (same as create)
- **Response:** `200 OK` (updated entry)

#### Delete Cash Entry

- **Endpoint:** `DELETE /entries/:id`
- **Description:** Remove a cash entry
- **Auth:** Required
- **URL Parameters:**
  - `id`: UUID of the cash entry
- **Response:** `200 OK`
  ```json
  {
    "message": "Entry deleted successfully"
  }
  ```

## Project Structure

```
cash-flow-forecast-backend/
├── main.go                  # Application entry point
├── go.mod                   # Go module definition
├── go.sum                   # Dependency checksums
├── Dockerfile               # Docker build configuration
├── .dockerignore             # Docker ignore rules
├── Makefile                 # Development tasks
├── .env.example             # Environment template
├── .gitignore               # Git ignore rules
├── README.md                # This file
├── internals/
│   ├── api/
│   │   └── routes.go        # API route definitions
│   ├── controllers/
│   │   ├── user.go          # User auth handlers
│   │   ├── cash.go          # Cash entry handlers
│   │   └── forecast.go      # Forecast logic and handlers
│   ├── db/
│   │   └── db.go            # Database connection
│   ├── helpers/
│   │   ├── auth.go          # JWT and auth utilities
│   │   └── password-helper.go # Password hashing
│   ├── middlewares/
│   │   ├── auth.go          # JWT validation middleware
│   │   └── cors.go          # CORS middleware
│   ├── migrate/
│   │   └── migrate.go       # Database migrations
│   └── models/
│       ├── user-model.go    # User schema
│       ├── cash-model.go    # CashEntry schema
│       └── forecast-model.go # Forecast schema
```

## Available Commands

```bash
# Build
make build                  # Compile the application
make clean                  # Remove build artifacts

# Development
make run                    # Run the application
make dev                    # Run with hot reload

# Testing
make test                   # Run all tests
make test-coverage          # Generate coverage report

# Code Quality
make fmt                    # Format code
make lint                   # Run linter
make deps-tidy              # Tidy dependencies

# Docker
make docker-build           # Build Docker image
make docker-run             # Run in Docker
make docker-compose-up      # Start with docker-compose
make docker-compose-down    # Stop docker-compose services
```

## Error Handling

The API returns standard HTTP status codes:

- `200 OK` — Successful request
- `201 Created` — Resource created
- `400 Bad Request` — Invalid input
- `401 Unauthorized` — Missing/invalid authentication
- `404 Not Found` — Resource not found
- `409 Conflict` — Resource already exists (e.g., duplicate email)
- `500 Internal Server Error` — Server error

Error responses include a descriptive message:

```json
{
  "error": "Invalid user ID"
}
```

## Security Considerations

- Passwords are hashed using bcrypt
- JWT tokens expire after 24 hours
- Auth cookies are HTTP-only to prevent XSS
- CORS is configured to prevent unauthorized cross-origin requests
- UUIDs are used for collision-resistant identifiers
- Environment variables store sensitive data (JWT_SECRET, DB_URL)

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/your-feature`)
3. Commit changes (`git commit -am 'Add your feature'`)
4. Push to branch (`git push origin feature/your-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License — see LICENSE file for details.

## Support

For issues, questions, or suggestions, please open an issue on the repository or contact the development team.
