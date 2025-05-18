# Balance Checker

A Go application to check token balances for multiple wallets across various blockchain networks. 
It aims to fetch token prices (future feature) and calculate portfolio value in USD (future feature).

## Features

- Fetches native and ERC20 token balances for specified wallets.
- Supports multiple EVM-compatible blockchain networks.
- Network configurations are hardcoded but easily extendable (`internal/infrastructure/network/definition/providers.go`).
- Token definitions are loaded from JSON files per network in `data/tokens/` (e.g., `data/tokens/ethereum.json`).
- Wallet addresses are loaded from `data/wallets.txt`.
- Concurrent fetching of balances with configurable limits.

## Configuration (`config/config.yml`)

- `server.port`: (Currently unused as there is no HTTP server) Port for the future HTTP server.
- `server.tracked_networks`: Comma-separated string of network short names to track (e.g., "ethereum,bsc,polygon"). 
  These short names must correspond to definitions in `providers.go` and have a token file in `data/tokens/` (e.g., `ethereum.json`).
- `database`: Configuration for PostgreSQL (currently not used for balance fetching, planned for storage).
- `logging.level`: Log level (e.g., "debug", "info", "warn", "error").
- `coingecko`: API key and cache settings for CoinGecko (price fetching is a future feature).
- `performance.max_concurrent_routines`: Limits the number of concurrent goroutines used for fetching balances across all wallets and networks.

## Project Structure

- `cmd/checker/main.go`: Main application entry point.
- `internal/`:
    - `app/`: Application core (services, ports/interfaces, providers).
        - `port/`: Interfaces defining contracts between application and infrastructure.
        - `provider/`: Implementations of some data provider interfaces.
        - `service/`: Business logic and use cases (e.g., `PortfolioService`).
    - `domain/entity/`: Core domain models (Wallet, TokenInfo, Balance, etc.).
    - `infrastructure/`: External concerns (network clients, data loaders, config loader).
        - `configloader/`: Loads `config.yml`.
        - `network/`:
            - `client/`: Blockchain client implementations (e.g., `EVMClient`).
            - `definition/`: Network definitions and providers.
        - `tokenloader/`: Loads token definitions from `data/tokens/`.
        - `walletloader/`: Loads wallet addresses from `data/wallets.txt`.
    - `pkg/`: Shared utility packages.
        - `logger/`: Application logger setup.
        - `utils/`: General utility functions (e.g., `FormatBigInt`).
- `config/`: Configuration files (`config.yml`).
- `data/`:
    - `tokens/`: JSON files with token definitions per network (e.g., `ethereum.json`, `bsc.json`).
        - **Token File Format**: Array of JSON objects, each with `chainId` (uint64), `address` (string), `name` (string), `symbol` (string), `decimals` (uint8).
    - `wallets.txt`: Plain text file with one wallet address per line. Lines starting with `#` are ignored.
- `Dockerfile`: For building the application container.
- `docker-compose.yml`: For running the application (and future database) via Docker Compose.

## How to Run

### Prerequisites

- Go (version 1.22+ recommended, see `go.mod`).
- Docker & Docker Compose (optional, for containerized execution).

### Setup

1.  **Clone the repository.**
2.  **Install dependencies:**
    ```bash
    go mod tidy
    ```
3.  **Configure Networks & Tokens:**
    *   Review and update RPC URLs in `internal/infrastructure/network/definition/providers.go` if necessary (especially `YOUR_INFURA_PROJECT_ID` for Ethereum).
    *   Ensure `data/tokens/` contains JSON files for the networks you want to track (e.g., `ethereum.json`). Verify `chainId` in these files matches the network definitions.
    *   Add wallet addresses to `data/wallets.txt`.
4.  **Configure Application:**
    *   Copy `config/config.yml.example` to `config/config.yml` if it doesn't exist (though the file is already present).
    *   Edit `config/config.yml`:
        *   Set `server.tracked_networks` to a comma-separated list of network short names you want to process (e.g., "ethereum,bsc").
        *   Adjust `performance.max_concurrent_routines` if needed.
        *   Set `logging.level` (e.g., "debug" for more details).

### Running Manually

```bash
go run cmd/checker/main.go
```

### Running with Docker Compose

```bash
docker-compose up --build
```
The application will run, fetch balances, and log the output.

## Development

- The project aims to follow Clean Architecture and DDD Lite principles.
- Key dependencies: `go-ethereum`, `slog` (standard library), `gopkg.in/yaml.v3`.

## TODO / Future Enhancements

- Fetch token prices from CoinGecko.
- Calculate portfolio value in USD.
- Store fetched balances and portfolio history in PostgreSQL.
- Expose an HTTP API (e.g., using Gin) to trigger fetches and query data.
- More robust error handling and retry mechanisms for RPC calls.
- Add support for more blockchain networks (e.g., Solana, non-EVM chains).
- Comprehensive unit and integration tests.
- Add address validation for wallets.
- Improve formatting of `FormatBigInt` for very small or very large numbers. 