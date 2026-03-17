package setup

import (
	"net/url"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/viper"
	"github.com/storacha/go-ucanto/did"
	"github.com/stretchr/testify/require"

	"github.com/storacha/piri/pkg/config"
	appcfg "github.com/storacha/piri/pkg/config/app"
)

// setupViperDefaults sets up viper with default values for tests.
// This simulates what loadPresetsAndConfig() does with network presets.
func setupViperDefaults(t *testing.T) {
	t.Helper()
	viper.Reset()
	t.Cleanup(viper.Reset)

	// Network
	viper.Set("network", "testnet")

	// PDP signing service
	viper.Set("pdp.signing_service.did", "did:key:signing")
	viper.Set("pdp.signing_service.url", "https://signing.example.com")

	// PDP contracts
	viper.Set("pdp.contracts.verifier", "0x1234567890123456789012345678901234567890")
	viper.Set("pdp.contracts.provider_registry", "0x2345678901234567890123456789012345678901")
	viper.Set("pdp.contracts.service", "0x3456789012345678901234567890123456789012")
	viper.Set("pdp.contracts.service_view", "0x4567890123456789012345678901234567891234")
	viper.Set("pdp.contracts.payments", "0x5678901234567890123456789012345678901234")
	viper.Set("pdp.contracts.usdfc_token", "0x6789012345678901234567890123456789012345")
	viper.Set("pdp.chain_id", "314159")
	viper.Set("pdp.payer_address", "0x7890123456789012345678901234567890123456")

	// UCAN services
	viper.Set("ucan.services.indexer.did", "did:key:indexer")
	viper.Set("ucan.services.indexer.url", "https://indexer.example.com")
	viper.Set("ucan.services.etracker.did", "did:key:etracker")
	viper.Set("ucan.services.etracker.url", "https://etracker.example.com")
	viper.Set("ucan.services.upload.did", "did:web:up.test.storacha.network")
	viper.Set("ucan.services.upload.url", "https://upload.example.com")
	viper.Set("ucan.services.publisher.ipni_announce_urls", []string{"https://ipni.example.com"})
	viper.Set("ucan.services.principal_mapping", map[string]string{"key": "value"})
}

func TestGenerateConfig(t *testing.T) {
	// Common test fixtures
	baseFlags := func() *initFlags {
		publicURL, _ := url.Parse("https://example.com")
		uploadDID, _ := did.Parse("did:web:up.test.storacha.network")
		return &initFlags{
			keyFile:   "/path/to/key.pem",
			publicURL: publicURL,
			baseConfig: &baseConfigValues{
				network:                 "testnet",
				signingServiceDID:       "did:key:signing",
				signingServiceURL:       "https://signing.example.com",
				uploadServiceDID:        uploadDID,
				uploadServiceURL:        "https://upload.example.com",
				verifierAddress:         "0x1234567890123456789012345678901234567890",
				providerRegistryAddress: "0x2345678901234567890123456789012345678901",
				serviceAddress:          "0x3456789012345678901234567890123456789012",
				serviceViewAddress:      "0x4567890123456789012345678901234567891234",
				paymentsAddress:         "0x5678901234567890123456789012345678901234",
				usdfcAddress:            "0x6789012345678901234567890123456789012345",
				chainID:                 "314159",
				payerAddress:            "0x7890123456789012345678901234567890123456",
				indexingServiceDID:      "did:key:indexer",
				indexingServiceURL:      "https://indexer.example.com",
				egressTrackerServiceDID: "did:key:etracker",
				egressTrackerServiceURL: "https://etracker.example.com",
				ipniAnnounceURLs:        []string{"https://ipni.example.com"},
				principalMapping:        map[string]string{"key": "value"},
			},
		}
	}

	baseCfg := func() *appcfg.AppConfig {
		return &appcfg.AppConfig{
			Storage: appcfg.StorageConfig{
				DataDir: "/data",
				TempDir: "/tmp",
			},
			Server: appcfg.ServerConfig{
				Host: "localhost",
				Port: 3000,
			},
		}
	}

	ownerAddress := common.HexToAddress("0x1234567890123456789012345678901234567890")

	t.Run("nil storage uses defaults", func(t *testing.T) {
		setupViperDefaults(t)
		flags := baseFlags()
		flags.storage = nil

		result, err := generateConfig(baseCfg(), flags, ownerAddress, 1, "indexer-proof", "egress-proof")
		require.NoError(t, err)

		// Database should have empty type (defaults to sqlite)
		require.Equal(t, "", result.Repo.Database.Type)
		// S3 should be nil
		require.Nil(t, result.Repo.S3)
	})

	t.Run("sqlite database configuration", func(t *testing.T) {
		setupViperDefaults(t)
		flags := baseFlags()
		flags.storage = &storageConfig{
			database: config.DatabaseConfig{
				Type: "sqlite",
			},
		}

		result, err := generateConfig(baseCfg(), flags, ownerAddress, 1, "indexer-proof", "egress-proof")
		require.NoError(t, err)

		require.Equal(t, "sqlite", result.Repo.Database.Type)
		require.Nil(t, result.Repo.S3)
	})

	t.Run("postgres database configuration", func(t *testing.T) {
		setupViperDefaults(t)
		flags := baseFlags()
		flags.storage = &storageConfig{
			database: config.DatabaseConfig{
				Type: "postgres",
				Postgres: config.PostgresConfig{
					URL:             "postgres://user:pass@localhost:5432/piri?sslmode=disable",
					MaxOpenConns:    10,
					MaxIdleConns:    5,
					ConnMaxLifetime: "1h",
				},
			},
		}

		result, err := generateConfig(baseCfg(), flags, ownerAddress, 1, "indexer-proof", "egress-proof")
		require.NoError(t, err)

		require.Equal(t, "postgres", result.Repo.Database.Type)
		require.Equal(t, "postgres://user:pass@localhost:5432/piri?sslmode=disable", result.Repo.Database.Postgres.URL)
		require.Equal(t, 10, result.Repo.Database.Postgres.MaxOpenConns)
		require.Equal(t, 5, result.Repo.Database.Postgres.MaxIdleConns)
		require.Equal(t, "1h", result.Repo.Database.Postgres.ConnMaxLifetime)
		require.Nil(t, result.Repo.S3)
	})

	t.Run("S3 storage configuration", func(t *testing.T) {
		setupViperDefaults(t)
		flags := baseFlags()
		flags.storage = &storageConfig{
			s3: &config.S3Config{
				Endpoint:     "minio.example.com:9000",
				BucketPrefix: "piri-",
				Credentials: config.Credentials{
					AccessKeyID:     "minioadmin",
					SecretAccessKey: "minioadmin123",
				},
				Insecure: true,
			},
		}

		result, err := generateConfig(baseCfg(), flags, ownerAddress, 1, "indexer-proof", "egress-proof")
		require.NoError(t, err)

		require.NotNil(t, result.Repo.S3)
		require.Equal(t, "minio.example.com:9000", result.Repo.S3.Endpoint)
		require.Equal(t, "piri-", result.Repo.S3.BucketPrefix)
		require.Equal(t, "minioadmin", result.Repo.S3.Credentials.AccessKeyID)
		require.Equal(t, "minioadmin123", result.Repo.S3.Credentials.SecretAccessKey)
		require.True(t, result.Repo.S3.Insecure)
	})

	t.Run("postgres and S3 combined configuration", func(t *testing.T) {
		setupViperDefaults(t)
		flags := baseFlags()
		flags.storage = &storageConfig{
			database: config.DatabaseConfig{
				Type: "postgres",
				Postgres: config.PostgresConfig{
					URL:             "postgres://user:pass@localhost:5432/piri",
					MaxOpenConns:    25,
					MaxIdleConns:    10,
					ConnMaxLifetime: "30m",
				},
			},
			s3: &config.S3Config{
				Endpoint:     "s3.amazonaws.com",
				BucketPrefix: "prod-piri-",
				Credentials: config.Credentials{
					AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
					SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
				},
				Insecure: false,
			},
		}

		result, err := generateConfig(baseCfg(), flags, ownerAddress, 1, "indexer-proof", "egress-proof")
		require.NoError(t, err)

		// Verify postgres config
		require.Equal(t, "postgres", result.Repo.Database.Type)
		require.Equal(t, "postgres://user:pass@localhost:5432/piri", result.Repo.Database.Postgres.URL)
		require.Equal(t, 25, result.Repo.Database.Postgres.MaxOpenConns)
		require.Equal(t, 10, result.Repo.Database.Postgres.MaxIdleConns)

		// Verify S3 config
		require.NotNil(t, result.Repo.S3)
		require.Equal(t, "s3.amazonaws.com", result.Repo.S3.Endpoint)
		require.Equal(t, "prod-piri-", result.Repo.S3.BucketPrefix)
		require.Equal(t, "AKIAIOSFODNN7EXAMPLE", result.Repo.S3.Credentials.AccessKeyID)
		require.False(t, result.Repo.S3.Insecure)
	})

	t.Run("verifies other config fields are populated", func(t *testing.T) {
		setupViperDefaults(t)
		flags := baseFlags()
		flags.storage = nil

		result, err := generateConfig(baseCfg(), flags, ownerAddress, 42, "indexer-proof", "egress-proof")
		require.NoError(t, err)

		// Verify key file is set
		require.Equal(t, "/path/to/key.pem", result.Identity.KeyFile)

		// Verify data directories from AppConfig.Storage
		require.Equal(t, "/data", result.Repo.DataDir)
		require.Equal(t, "/tmp", result.Repo.TempDir)

		// Verify server config
		require.Equal(t, "localhost", result.Server.Host)
		require.Equal(t, uint(3000), result.Server.Port)
		require.Equal(t, "https://example.com", result.Server.PublicURL)

		// Verify proof set ID
		require.Equal(t, uint64(42), result.UCANService.ProofSetID)

		// Verify proofs
		require.Equal(t, "indexer-proof", result.UCANService.Services.Indexer.Proof)
		require.Equal(t, "egress-proof", result.UCANService.Services.EgressTracker.Proof)

		// Verify network is read from viper
		require.Equal(t, "testnet", result.Network)
	})

	// Test cases for merged storage configuration
	// With viper-based config merging, parseAndValidateFlags merges CLI flags with base-config
	// and populates flags.storage with the merged result. generateConfig then uses flags.storage
	// directly. These tests verify that generateConfig correctly handles the merged config.

	t.Run("merged config: postgres from flags, S3 from base-config", func(t *testing.T) {
		setupViperDefaults(t)
		flags := baseFlags()
		// After viper merge in parseAndValidateFlags, storage contains both postgres and S3
		// - postgres values came from CLI flags (higher priority)
		// - S3 values came from base-config (lower priority)
		flags.storage = &storageConfig{
			database: config.DatabaseConfig{
				Type: "postgres",
				Postgres: config.PostgresConfig{
					URL:             "postgres://flags:user@localhost:5432/piri",
					MaxOpenConns:    20,
					MaxIdleConns:    10,
					ConnMaxLifetime: "1h",
				},
			},
			s3: &config.S3Config{
				Endpoint:     "base-s3.example.com:9000",
				BucketPrefix: "base-",
				Credentials: config.Credentials{
					AccessKeyID:     "basekey",
					SecretAccessKey: "basesecret",
				},
				Insecure: false,
			},
		}

		result, err := generateConfig(baseCfg(), flags, ownerAddress, 1, "indexer-proof", "egress-proof")
		require.NoError(t, err)

		// Both postgres and S3 should be present in the output
		require.Equal(t, "postgres", result.Repo.Database.Type)
		require.Equal(t, "postgres://flags:user@localhost:5432/piri", result.Repo.Database.Postgres.URL)
		require.Equal(t, 20, result.Repo.Database.Postgres.MaxOpenConns)

		require.NotNil(t, result.Repo.S3)
		require.Equal(t, "base-s3.example.com:9000", result.Repo.S3.Endpoint)
		require.Equal(t, "base-", result.Repo.S3.BucketPrefix)
	})

	t.Run("merged config: S3 from flags, postgres from base-config", func(t *testing.T) {
		setupViperDefaults(t)
		flags := baseFlags()
		// After viper merge in parseAndValidateFlags, storage contains both S3 and postgres
		// - S3 values came from CLI flags (higher priority)
		// - postgres values came from base-config (lower priority)
		flags.storage = &storageConfig{
			database: config.DatabaseConfig{
				Type: "postgres",
				Postgres: config.PostgresConfig{
					URL:             "postgres://base:config@localhost:5432/piri",
					MaxOpenConns:    12,
					MaxIdleConns:    6,
					ConnMaxLifetime: "30m",
				},
			},
			s3: &config.S3Config{
				Endpoint:     "flags-s3.example.com:9000",
				BucketPrefix: "flags-",
				Credentials: config.Credentials{
					AccessKeyID:     "flagskey",
					SecretAccessKey: "flagssecret",
				},
				Insecure: true,
			},
		}

		result, err := generateConfig(baseCfg(), flags, ownerAddress, 1, "indexer-proof", "egress-proof")
		require.NoError(t, err)

		// Both S3 and postgres should be present in the output
		require.NotNil(t, result.Repo.S3)
		require.Equal(t, "flags-s3.example.com:9000", result.Repo.S3.Endpoint)
		require.Equal(t, "flags-", result.Repo.S3.BucketPrefix)
		require.True(t, result.Repo.S3.Insecure)

		require.Equal(t, "postgres", result.Repo.Database.Type)
		require.Equal(t, "postgres://base:config@localhost:5432/piri", result.Repo.Database.Postgres.URL)
	})

	t.Run("merged config: flags override base-config for same field", func(t *testing.T) {
		setupViperDefaults(t)
		flags := baseFlags()
		// After viper merge, flag values take precedence over base-config values
		// for the same fields
		flags.storage = &storageConfig{
			database: config.DatabaseConfig{
				Type: "postgres",
				Postgres: config.PostgresConfig{
					URL:             "postgres://flags:winner@localhost:5432/piri",
					MaxOpenConns:    50,
					MaxIdleConns:    25,
					ConnMaxLifetime: "2h",
				},
			},
			s3: &config.S3Config{
				Endpoint:     "flags-s3.example.com:9000",
				BucketPrefix: "flags-winner-",
			},
		}

		result, err := generateConfig(baseCfg(), flags, ownerAddress, 1, "indexer-proof", "egress-proof")
		require.NoError(t, err)

		// Flag values should be used (they had higher priority during viper merge)
		require.Equal(t, "postgres://flags:winner@localhost:5432/piri", result.Repo.Database.Postgres.URL)
		require.Equal(t, 50, result.Repo.Database.Postgres.MaxOpenConns)
		require.Equal(t, "flags-s3.example.com:9000", result.Repo.S3.Endpoint)
		require.Equal(t, "flags-winner-", result.Repo.S3.BucketPrefix)
	})
}
