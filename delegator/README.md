# Delegator HTTP Server

A simple HTTP server built with:
- Cobra for CLI
- Viper for configuration
- Uber.fx for dependency injection
- Echo for HTTP server

## Quick Start

```bash
# Build the server
go build -o delegator

# Run the server
./delegator serve

# Or with custom host/port
./delegator serve --host 127.0.0.1 --port 9090
```

## Endpoints

- `GET /` - Returns "hello"
- `GET /health` - Health check endpoint
- `PUT /register` - Registration endpoint (payload TBD)
- `GET /request-proof` - Request proof endpoint (payload TBD)

## Configuration

Copy `.delegator.example.yaml` to `.delegator.yaml` and modify as needed.

The server can be configured via:
- Command line flags
- Environment variables
- Configuration file (.delegator.yaml)