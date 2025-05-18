# Balance Checker API

A Go application to check token balances for multiple wallets across various blockchain networks.

## Features

- Fetches token balances for specified wallets.
- Retrieves token prices from CoinGecko API.
- Calculates portfolio value in USD.
- Supports multiple blockchain networks (configurable).
- Uses PostgreSQL for data storage.
- Dockerized for easy deployment.

## Project Structure

```
balance_checker/
├── cmd/parserapi/main.go       # Main application entry point
├── internal/                   # Internal application code (Clean Architecture)
│   ├── app/                    # Application layer (use cases, DTOs)
│   ├── domain/                 # Domain layer (entities, value objects)
│   └── infrastructure/         # Infrastructure layer (DB, external APIs, HTTP)
├── config/config.yml           # Configuration file
├── data/
│   ├── wallets.txt             # List of wallet addresses
│   └── tokens.json             # Token definitions
├── migrations/                 # Database migrations
├── pkg/                        # Shared utility packages
├── scripts/                    # Build/run scripts
├── test/                       # Tests
├── .gitignore
├── Dockerfile
├── docker-compose.yml
├── go.mod
├── go.sum
└── README.md
```

## Getting Started

### Prerequisites

- Go (version 1.24 or later)
- Docker & Docker Compose
- PostgreSQL

### Installation & Running

1.  **Clone the repository:**
    ```bash
    git clone <repository-url>
    cd balance_checker
    ```

2.  **Configuration:**
    *   Copy `config/config.example.yml` to `config/config.yml` (if example is provided) and update with your database credentials, CoinGecko API key, etc.
    *   Create `data/wallets.txt` with one wallet address per line.
    *   Create `data/tokens.json` with the list of tokens to track.

3.  **Using Docker Compose (Recommended):**
    ```bash
    docker-compose up --build
    ```
    The API will be available at `http://localhost:8080` (or as configured).

4.  **Running Manually (for development):**
    ```bash
    # Initialize Go modules (if needed)
    go mod tidy

    # Run database migrations (details musculaire)

    # Run the application
    go run cmd/parserapi/main.go -config=config/config.yml
    ```

## API Endpoints

- `GET /wallets/{address}/portfolio`: Get portfolio for a specific wallet.
- (More endpoints to be defined)

## TODO

- Implement core logic for balance fetching and price aggregation.
- Define database schema and write migrations.
- Implement API handlers and use cases.
- Write unit and integration tests. 