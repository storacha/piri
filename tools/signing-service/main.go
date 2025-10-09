package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	"github.com/spf13/cobra"
	"github.com/storacha/piri/tools/service-operator/eip712"
	"github.com/storacha/piri/tools/signing-service/config"
	"github.com/storacha/piri/tools/signing-service/handlers"
)

var (
	port             int
	privateKeyPath   string
	keystorePath     string
	keystorePassword string
	rpcUrl           string
	contractAddress  string
	network          string
)

var rootCmd = &cobra.Command{
	Use:   "signing-service",
	Short: "HTTP service for signing PDP operations on behalf of Storacha",
	Long: `A signing service that accepts PDP operation payloads via HTTP and returns
EIP-712 signatures. This service wraps the eip712.Signer and provides a REST API
for piri nodes to request signatures without exposing Storacha's private key.

Phase 1 (current): Blindly signs any request (no authentication)
Phase 2 (future): UCAN authentication for registered operators
Phase 3 (future): Session key integration
Phase 4 (future): Replace cold wallet with session key`,
	RunE: run,
}

func init() {
	rootCmd.Flags().IntVar(&port, "port", 8080, "HTTP server port")
	rootCmd.Flags().StringVar(&privateKeyPath, "private-key", "", "Path to private key file")
	rootCmd.Flags().StringVar(&keystorePath, "keystore", "", "Path to keystore file")
	rootCmd.Flags().StringVar(&keystorePassword, "keystore-password", "", "Keystore password")
	rootCmd.Flags().StringVar(&rpcUrl, "rpc-url", "", "Ethereum RPC URL")
	rootCmd.Flags().StringVar(&contractAddress, "contract-address", "", "FilecoinWarmStorageService contract address")
	rootCmd.Flags().StringVar(&network, "network", "", "Network to use (calibration or mainnet)")
}

func run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	// Load private key
	var privateKey *ecdsa.PrivateKey
	if cfg.PrivateKeyPath != "" {
		privateKey, err = config.LoadPrivateKey(cfg.PrivateKeyPath)
		if err != nil {
			return fmt.Errorf("loading private key: %w", err)
		}
	} else {
		privateKey, err = config.LoadPrivateKeyFromKeystore(cfg.KeystorePath, cfg.KeystorePassword)
		if err != nil {
			return fmt.Errorf("loading keystore: %w", err)
		}
	}

	// Connect to RPC to get chain ID
	client, err := ethclient.Dial(cfg.RPCUrl)
	if err != nil {
		return fmt.Errorf("connecting to RPC endpoint: %w", err)
	}
	defer client.Close()

	chainID, err := client.ChainID(ctx)
	if err != nil {
		return fmt.Errorf("getting chain ID: %w", err)
	}

	// Create EIP-712 signer
	signer := eip712.NewSigner(privateKey, chainID, cfg.ContractAddr())

	// Create Echo instance
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Logger.SetLevel(log.DEBUG)

	// Create HTTP handlers
	handler := handlers.NewHandler(signer)

	// Setup routes
	e.GET("/health", handler.Health)
	e.POST("/sign/create-dataset", handler.SignCreateDataSet)
	e.POST("/sign/add-pieces", handler.SignAddPieces)
	e.POST("/sign/schedule-piece-removals", handler.SignSchedulePieceRemovals)
	e.POST("/sign/delete-dataset", handler.SignDeleteDataSet)

	// Log startup info
	cmd.Println("Signing service starting...")
	cmd.Printf("  Signer address: %s", signer.GetAddress().Hex())
	cmd.Printf("  Chain ID: %s", chainID.String())
	cmd.Printf("  Verifying contract: %s", cfg.ContractAddress)
	cmd.Printf("  Port: %d", port)
	cmd.Printf("⚠️  WARNING: This service blindly signs any request (no authentication)")

	// Start server in goroutine
	go func() {
		e.Logger.Infof("✓ Server listening on http://localhost:%d", port)
		if err := e.Start(fmt.Sprintf(":%d", port)); err != nil && err != http.ErrServerClosed {
			e.Logger.Fatal("shutting down the server")
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	e.Logger.Info("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := e.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown error: %w", err)
	}

	e.Logger.Info("Server stopped")
	return nil
}

func loadConfig() (*config.Config, error) {
	cfg := &config.Config{
		PrivateKeyPath:   privateKeyPath,
		KeystorePath:     keystorePath,
		KeystorePassword: keystorePassword,
		RPCUrl:           rpcUrl,
		ContractAddress:  contractAddress,
		Network:          network,
	}

	// If network is specified, use network defaults
	if network != "" {
		defaultRPC, defaultAddr, err := config.NetworkDefaults(network)
		if err != nil {
			return nil, err
		}
		if cfg.RPCUrl == "" {
			cfg.RPCUrl = defaultRPC
		}
		if cfg.ContractAddress == "" {
			cfg.ContractAddress = defaultAddr
		}
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("configuration error: %w", err)
	}

	return cfg, nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
