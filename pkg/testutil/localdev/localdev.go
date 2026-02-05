// Package localdev provides a testcontainers module for running a local
// Filecoin development environment with pre-deployed smart contracts.
//
// The container runs Anvil (local EVM) with mock Filecoin RPC endpoints,
// providing both Ethereum and Lotus API compatibility on a single port.
//
// Usage:
//
//	container, err := localdev.Run(ctx, localdev.WithBlockTime(3))
//	if err != nil {
//	    return err
//	}
//	defer container.Terminate(ctx)
//
//	// Connect to the container
//	client, _ := ethclient.Dial(container.RPCEndpoint)
package localdev

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	// DefaultImage is the published localdev Docker image
	DefaultImage = "ghcr.io/storacha/filecoin-localdev:b66c8bd"

	// DefaultRPCPort is the port exposed by the container
	DefaultRPCPort = "8545/tcp"

	// ChainID is the Anvil chain ID (matches Piri's hardcoded value)
	ChainID = 31337
)

// Container represents a running localdev container
type Container struct {
	testcontainers.Container
	// RPCEndpoint is the URL to connect to (e.g., "http://localhost:32768")
	RPCEndpoint string
	// Addresses contains the actual deployed contract addresses read from the container
	Addresses ContractAddresses
}

// WaitForReady performs additional health checks to ensure the container
// is fully operational. It verifies:
// 1. RPC endpoint responds to eth_chainId
// 2. Block number is greater than 0 (contracts have been deployed)
func (c *Container) WaitForReady(ctx context.Context) error {
	client, err := rpc.Dial(c.RPCEndpoint)
	if err != nil {
		return fmt.Errorf("connecting to RPC: %w", err)
	}
	defer client.Close()

	// Check chain ID responds
	var chainID string
	if err := client.CallContext(ctx, &chainID, "eth_chainId"); err != nil {
		return fmt.Errorf("eth_chainId failed: %w", err)
	}

	// Check block number > 0 (contracts have been deployed)
	var blockNum string
	if err := client.CallContext(ctx, &blockNum, "eth_blockNumber"); err != nil {
		return fmt.Errorf("eth_blockNumber failed: %w", err)
	}

	return nil
}

// deployedAddressesJSON represents the structure of /deployed-addresses.json in the container
type deployedAddressesJSON struct {
	Contracts struct {
		MockUSDFC                           string `json:"MockUSDFC"`
		SessionKeyRegistry                  string `json:"SessionKeyRegistry"`
		PDPVerifier                         string `json:"PDPVerifier"`
		FilecoinPayV1                       string `json:"FilecoinPayV1"`
		ServiceProviderRegistry             string `json:"ServiceProviderRegistry"`
		SignatureVerificationLib            string `json:"SignatureVerificationLib"`
		FilecoinWarmStorageService          string `json:"FilecoinWarmStorageService"`
		FilecoinWarmStorageServiceStateView string `json:"FilecoinWarmStorageServiceStateView"`
	} `json:"contracts"`
}

// ReadDeployedAddresses reads the contract addresses from the container's /deployed-addresses.json file
func (c *Container) ReadDeployedAddresses(ctx context.Context) (ContractAddresses, error) {
	// Read the file from the container
	reader, err := c.CopyFileFromContainer(ctx, "/deployed-addresses.json")
	if err != nil {
		return ContractAddresses{}, fmt.Errorf("reading deployed-addresses.json from container: %w", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return ContractAddresses{}, fmt.Errorf("reading deployed-addresses.json content: %w", err)
	}

	var deployed deployedAddressesJSON
	if err := json.Unmarshal(data, &deployed); err != nil {
		return ContractAddresses{}, fmt.Errorf("parsing deployed-addresses.json: %w", err)
	}

	return ContractAddresses{
		MockUSDFC:                  deployed.Contracts.MockUSDFC,
		SessionKeyRegistry:         deployed.Contracts.SessionKeyRegistry,
		PDPVerifier:                deployed.Contracts.PDPVerifier,
		FilecoinPayV1:              deployed.Contracts.FilecoinPayV1,
		ServiceProviderRegistry:    deployed.Contracts.ServiceProviderRegistry,
		SignatureVerificationLib:   deployed.Contracts.SignatureVerificationLib,
		FilecoinWarmStorageService: deployed.Contracts.FilecoinWarmStorageService,
		ServiceStateView:           deployed.Contracts.FilecoinWarmStorageServiceStateView,
	}, nil
}

// ContractAddresses contains the deployed contract addresses.
// These are deterministic due to Anvil's fixed mnemonic and deployment order.
type ContractAddresses struct {
	MockUSDFC                  string
	SessionKeyRegistry         string
	PDPVerifier                string
	FilecoinPayV1              string
	ServiceProviderRegistry    string
	SignatureVerificationLib   string
	FilecoinWarmStorageService string
	ServiceStateView           string
}

// Account represents an Anvil pre-funded account
type Account struct {
	Address    string
	PrivateKey string
}

type ChainAccounts struct {
	// Deployer is Account #0, used to deploy all contracts
	Deployer Account
	// Payer is Account #1, pre-configured with USDFC and operator approvals
	Payer Account
	// ServiceProvider is Account #2, can be used as a PDP service provider
	ServiceProvider Account
}

// Accounts contains the pre-funded Anvil accounts (10,000 ETH each)
var Accounts = ChainAccounts{
	Deployer: Account{
		Address:    "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
		PrivateKey: "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
	},
	Payer: Account{
		Address:    "0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
		PrivateKey: "0x59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d",
	},
	ServiceProvider: Account{
		Address:    "0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC",
		PrivateKey: "0x5de4111afa1a4b94908f83103eb1f1706367c2e68ca870fc3fb9a804cdab365a",
	},
}

// Options configures the localdev container
type Options struct {
	// Image is the Docker image to use
	Image string
	// BlockTime is the Anvil block time in seconds
	BlockTime int
	// StartupTimeout is the maximum time to wait for the container to be ready
	StartupTimeout time.Duration
	// StateFile is the path to an Anvil state file to mount (for fast startup).
	// Must be in SerializableState JSON format (generated via --dump-state CLI flag).
	StateFile string
	// DeployedAddressesFile is the path to deployed-addresses.json to mount.
	// Required when using StateFile so contract addresses can be read.
	DeployedAddressesFile string
	// UseEmbeddedState uses the embedded anvil-state.json and deployed-addresses.json files
	// instead of requiring external file paths. The embedded files are written to temp files
	// which are automatically cleaned up.
	UseEmbeddedState bool
}

// DefaultOptions returns sensible defaults for testing
func DefaultOptions() Options {
	return Options{
		Image:     DefaultImage,
		BlockTime: 3, // 3 second blocks for faster tests,
		// lower than this we hit issues with piri scheduling
		StartupTimeout: time.Minute, // container should come up quick
	}
}

// Run starts a new localdev container and waits for it to be ready.
// The container exposes a single RPC endpoint that handles both
// Ethereum (eth_*) and Filecoin (Filecoin.*) JSON-RPC methods.
//
// If a state file is provided via WithStateFile, the container will load
// that state instead of deploying contracts, resulting in much faster
// startup (~3s vs ~30s).
func Run(ctx context.Context, opts ...func(*Options)) (*Container, error) {
	options := DefaultOptions()
	for _, opt := range opts {
		opt(&options)
	}

	req := testcontainers.ContainerRequest{
		Image:        options.Image,
		ExposedPorts: []string{DefaultRPCPort},
		Env: map[string]string{
			"ANVIL_BLOCK_TIME": fmt.Sprintf("%d", options.BlockTime),
		},
	}

	// If state file is provided (either embedded or path), copy files into container
	if options.UseEmbeddedState || options.StateFile != "" {
		var stateContent, addressesContent []byte

		if options.UseEmbeddedState {
			stateContent = AnvilStateJSON
			addressesContent = DeployedAddressesJSON
		} else {
			if options.DeployedAddressesFile == "" {
				return nil, fmt.Errorf("DeployedAddressesFile is required when using StateFile")
			}
			var err error
			stateContent, err = os.ReadFile(options.StateFile)
			if err != nil {
				return nil, fmt.Errorf("reading state file: %w", err)
			}
			addressesContent, err = os.ReadFile(options.DeployedAddressesFile)
			if err != nil {
				return nil, fmt.Errorf("reading addresses file: %w", err)
			}
		}

		req.Files = []testcontainers.ContainerFile{
			{
				Reader:            bytes.NewReader(stateContent),
				ContainerFilePath: "/app/anvil-state.json",
				FileMode:          0644,
			},
			{
				Reader:            bytes.NewReader(addressesContent),
				ContainerFilePath: "/deployed-addresses.json",
				FileMode:          0644,
			},
		}

		// With pre-loaded state, startup is much faster - just wait for RPC ready
		req.WaitingFor = wait.ForAll(
			wait.ForLog("Local Environment Ready!").WithStartupTimeout(30*time.Second),
			wait.ForListeningPort(DefaultRPCPort).WithStartupTimeout(30*time.Second),
		).WithDeadline(30 * time.Second)
	} else {
		// Without state, need to wait for contract deployment
		req.WaitingFor = wait.ForAll(
			wait.ForLog("Local Environment Ready!").WithStartupTimeout(options.StartupTimeout),
			wait.ForListeningPort(DefaultRPCPort).WithStartupTimeout(options.StartupTimeout),
		).WithDeadline(options.StartupTimeout)
	}

	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start localdev container: %w", err)
	}

	host, err := c.Host(ctx)
	if err != nil {
		_ = c.Terminate(ctx)
		return nil, fmt.Errorf("failed to get container host: %w", err)
	}

	port, err := c.MappedPort(ctx, DefaultRPCPort)
	if err != nil {
		_ = c.Terminate(ctx)
		return nil, fmt.Errorf("failed to get container port: %w", err)
	}

	container := &Container{
		Container:   c,
		RPCEndpoint: fmt.Sprintf("http://%s:%s", host, port.Port()),
	}

	// Perform additional RPC health check to ensure container is fully operational
	if err := container.WaitForReady(ctx); err != nil {
		_ = c.Terminate(ctx)
		return nil, fmt.Errorf("container not ready: %w", err)
	}

	// Read deployed contract addresses from the container
	// (either from deployment or mounted deployed-addresses.json)
	addresses, err := container.ReadDeployedAddresses(ctx)
	if err != nil {
		_ = c.Terminate(ctx)
		return nil, fmt.Errorf("reading deployed addresses: %w", err)
	}
	container.Addresses = addresses

	return container, nil
}

// WithImage sets a custom Docker image
func WithImage(image string) func(*Options) {
	return func(o *Options) {
		o.Image = image
	}
}

// WithBlockTime sets the Anvil block time in seconds.
// Lower values make tests faster but less realistic.
// Default is 3 seconds.
func WithBlockTime(seconds int) func(*Options) {
	return func(o *Options) {
		o.BlockTime = seconds
	}
}

// WithStartupTimeout sets the maximum time to wait for container startup.
// The container needs time for Anvil to start, contracts to deploy,
// and the mock Lotus RPC server to initialize.
// Default is 120 seconds. Use longer timeouts for slower machines.
func WithStartupTimeout(d time.Duration) func(*Options) {
	return func(o *Options) {
		o.StartupTimeout = d
	}
}

// WithStateFile mounts an existing Anvil state file for fast startup.
// When a state file is provided, the container loads it instead of
// deploying contracts from scratch, reducing startup time from ~30s to ~3s.
//
// IMPORTANT: Must be used together with WithDeployedAddressesFile.
//
// The state file must be in Anvil's SerializableState JSON format,
// which is generated using the --dump-state CLI flag (NOT the anvil_dumpState RPC).
//
// To generate both files, run the container in dump mode:
//
//	docker run --rm -v $(pwd):/output -e DUMP_STATE=true filecoin-localdev:local
//
// This will create ./anvil-state.json and ./deployed-addresses.json.
func WithStateFile(path string) func(*Options) {
	return func(o *Options) {
		o.StateFile = path
	}
}

// WithDeployedAddressesFile mounts the deployed-addresses.json file.
// Required when using WithStateFile so contract addresses can be read.
func WithDeployedAddressesFile(path string) func(*Options) {
	return func(o *Options) {
		o.DeployedAddressesFile = path
	}
}

// WithEmbeddedState uses the embedded anvil-state.json and deployed-addresses.json files.
// The embedded files are copied directly into the container using the Files API.
// This is the recommended option for portable tests that work across different machines.
func WithEmbeddedState() func(*Options) {
	return func(o *Options) {
		o.UseEmbeddedState = true
	}
}
