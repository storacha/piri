package provider

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/cobra"

	"github.com/storacha/piri/pkg/pdp/smartcontracts"
	"github.com/storacha/piri/pkg/pdp/smartcontracts/bindings"
)

var (
	listLimit        uint64
	listOffset       uint64
	listShowInactive bool
	listFormat       string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered service providers",
	Long: `List service providers registered in the ServiceProviderRegistry.

By default, only active providers are shown. Use --show-inactive to include inactive providers.
Results can be paginated using --offset and --limit flags.`,
	Args: cobra.NoArgs,
	RunE: runList,
}

func init() {
	listCmd.Flags().Uint64Var(&listLimit, "limit", 50, "Maximum number of providers to display")
	listCmd.Flags().Uint64Var(&listOffset, "offset", 0, "Starting offset for pagination")
	listCmd.Flags().BoolVar(&listShowInactive, "show-inactive", false, "Include inactive providers")
	listCmd.Flags().StringVar(&listFormat, "format", "table", "Output format: table or json")
}

type ProviderInfo struct {
	ID          uint64 `json:"id"`
	Address     string `json:"address"`
	Payee       string `json:"payee"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IsActive    bool   `json:"isActive"`
	IsApproved  bool   `json:"isApproved"`
}

type ListResult struct {
	Providers []ProviderInfo `json:"providers"`
	HasMore   bool           `json:"hasMore"`
	Offset    uint64         `json:"offset"`
	Limit     uint64         `json:"limit"`
}

func runList(cobraCmd *cobra.Command, args []string) error {
	ctx := cobraCmd.Context()

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// For list command, we only need RPC URL (read-only operation)
	if cfg.RPCUrl == "" {
		return fmt.Errorf("--rpc-url is required")
	}

	if listFormat != "table" && listFormat != "json" {
		return fmt.Errorf("invalid format: %s (must be 'table' or 'json')", listFormat)
	}

	client, err := ethclient.Dial(cfg.RPCUrl)
	if err != nil {
		return fmt.Errorf("connecting to RPC endpoint: %w", err)
	}
	defer client.Close()

	registry, err := bindings.NewServiceProviderRegistry(smartcontracts.Addresses().ProviderRegistry, client)
	if err != nil {
		return fmt.Errorf("creating registry binding: %w", err)
	}

	bindCtx := &bind.CallOpts{Context: ctx}

	// Get provider IDs with pagination
	result, err := registry.GetAllActiveProviders(bindCtx, big.NewInt(int64(listOffset)), big.NewInt(int64(listLimit)))
	if err != nil {
		return fmt.Errorf("getting active providers: %w", err)
	}

	if len(result.ProviderIds) == 0 {
		if listFormat == "json" {
			output, _ := json.MarshalIndent(ListResult{
				Providers: []ProviderInfo{},
				HasMore:   false,
				Offset:    listOffset,
				Limit:     listLimit,
			}, "", "  ")
			fmt.Println(string(output))
		} else {
			fmt.Println("No providers found.")
		}
		return nil
	}

	// Get full provider information
	providersResult, err := registry.GetProvidersByIds(bindCtx, result.ProviderIds)
	if err != nil {
		return fmt.Errorf("getting provider details: %w", err)
	}

	// Try to connect to view contract to check approval status
	// This is optional - if contract address is not provided or view contract is not available,
	// we'll show approval status as false
	var viewHelper *smartcontracts.ViewContractHelper
	if cfg.ContractAddress != "" {
		viewHelper, err = smartcontracts.NewViewContractHelper(client, common.HexToAddress(cfg.ContractAddress))
		if err != nil {
			// Log warning but continue without approval status
			fmt.Fprintf(cobraCmd.ErrOrStderr(), "Warning: Could not connect to view contract (approval status will not be shown): %v\n", err)
		}
	}

	// Convert to display format
	providers := make([]ProviderInfo, 0)
	for i, providerView := range providersResult.ProviderInfos {
		if !providersResult.ValidIds[i] {
			continue
		}

		// Skip inactive providers unless requested
		if !listShowInactive && !providerView.Info.IsActive {
			continue
		}

		// Check approval status if view contract is available
		isApproved := false
		if viewHelper != nil {
			approved, err := viewHelper.IsProviderApproved(providerView.ProviderId)
			if err != nil {
				// Log warning but continue with false
				fmt.Fprintf(cobraCmd.ErrOrStderr(), "Warning: Could not check approval for provider %d: %v\n", providerView.ProviderId.Uint64(), err)
			} else {
				isApproved = approved
			}
		}

		providers = append(providers, ProviderInfo{
			ID:          providerView.ProviderId.Uint64(),
			Address:     providerView.Info.ServiceProvider.Hex(),
			Payee:       providerView.Info.Payee.Hex(),
			Name:        providerView.Info.Name,
			Description: providerView.Info.Description,
			IsActive:    providerView.Info.IsActive,
			IsApproved:  isApproved,
		})
	}

	// Display results
	if listFormat == "json" {
		output, err := json.MarshalIndent(ListResult{
			Providers: providers,
			HasMore:   result.HasMore,
			Offset:    listOffset,
			Limit:     listLimit,
		}, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling JSON: %w", err)
		}
		fmt.Println(string(output))
	} else {
		displayTable(providers, result.HasMore)
	}

	return nil
}

func displayTable(providers []ProviderInfo, hasMore bool) {
	if len(providers) == 0 {
		fmt.Println("No providers found.")
		return
	}

	// Calculate column widths
	maxID := 2
	maxAddress := 7
	maxPayee := 5
	maxName := 4
	maxDesc := 11

	for _, p := range providers {
		idLen := len(fmt.Sprintf("%d", p.ID))
		if idLen > maxID {
			maxID = idLen
		}
		if len(p.Address) > maxAddress {
			maxAddress = len(p.Address)
		}
		if len(p.Payee) > maxPayee {
			maxPayee = len(p.Payee)
		}
		if len(p.Name) > maxName {
			maxName = len(p.Name)
		}
		if len(p.Description) > maxDesc {
			maxDesc = len(p.Description)
		}
	}

	// Limit description width for table readability
	if maxDesc > 40 {
		maxDesc = 40
	}
	// Limit name width
	if maxName > 30 {
		maxName = 30
	}

	// Print header
	fmt.Printf("%-*s  %-*s  %-*s  %-*s  %-*s  %-8s  %-8s\n",
		maxID, "ID",
		maxAddress, "Address",
		maxPayee, "Payee",
		maxName, "Name",
		maxDesc, "Description",
		"Active",
		"Approved")

	fmt.Println(strings.Repeat("-", maxID+maxAddress+maxPayee+maxName+maxDesc+28))

	const (
		TrueSymbol  = "✅"
		FalseSymbol = "❌"
	)
	// Print rows
	for _, p := range providers {
		activeSymbol := FalseSymbol
		if p.IsActive {
			activeSymbol = TrueSymbol
		}

		approvedSymbol := FalseSymbol
		if p.IsApproved {
			approvedSymbol = TrueSymbol
		}

		name := p.Name
		if len(name) > maxName {
			name = name[:maxName-3] + "..."
		}

		desc := p.Description
		if len(desc) > maxDesc {
			desc = desc[:maxDesc-3] + "..."
		}

		fmt.Printf("%-*d  %-*s  %-*s  %-*s  %-*s  %-8s  %s\n",
			maxID, p.ID,
			maxAddress, p.Address,
			maxPayee, p.Payee,
			maxName, name,
			maxDesc, desc,
			activeSymbol,
			approvedSymbol)
	}

	fmt.Println()
	fmt.Printf("Showing %d provider(s)", len(providers))
	if hasMore {
		fmt.Printf(" (more available - use --offset %d to see next page)", listOffset+listLimit)
	}
	fmt.Println()
}
