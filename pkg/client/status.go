package client

import (
	"context"
	"fmt"
	"time"

	"github.com/storacha/piri/pkg/config"
	"github.com/storacha/piri/pkg/pdp/httpapi/client"
	"github.com/storacha/piri/pkg/pdp/types"
)

// NodeStatus represents the current state of a piri node
type NodeStatus struct {
	Healthy           bool       `json:"healthy"`
	IsProving         bool       `json:"is_proving"`
	InChallengeWindow bool       `json:"in_challenge_window"`
	HasProven         bool       `json:"has_proven"`
	InFaultState      bool       `json:"in_fault_state"`
	UpgradeSafe       bool       `json:"upgrade_safe"`
	NextChallenge     *time.Time `json:"next_challenge,omitempty"`
}

// GetNodeStatus connects to a running piri node and checks its status
func GetNodeStatus(ctx context.Context) (*NodeStatus, error) {
	userCfg, err := config.Load[config.FullServerConfig]()
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	appCfg, err := userCfg.ToAppConfig()
	if err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	// Create API client to connect to local node
	api, err := client.NewFromConfig(config.Client{
		Identity: userCfg.Identity,
		API: config.API{
			Endpoint: fmt.Sprintf("http://%s:%d", userCfg.Server.Host, userCfg.Server.Port),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("creating api client: %w", err)
	}

	// Get proof set state
	psState, err := api.GetProofSetState(ctx, appCfg.UCANService.ProofSetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get proof set state: %w", err)
	}

	// Calculate if it's safe to upgrade
	upgradeSafe := calculateUpgradeSafety(psState)

	return &NodeStatus{
		Healthy:           true, // If we got this far, node is responding
		IsProving:         psState.IsProving,
		InChallengeWindow: psState.InChallengeWindow,
		HasProven:         psState.HasProven,
		InFaultState:      psState.IsInFaultState,
		UpgradeSafe:       upgradeSafe,
		// NextChallenge could be calculated from psState if needed
	}, nil
}

// calculateUpgradeSafety determines if it's safe to update based on proof set state
func calculateUpgradeSafety(psState *types.ProofSetState) bool {
	// If in fault state, we cannot update
	// TODO/REVIEW: I don't know if we want to allow updates to nodes in fault.
	if psState.IsInFaultState {
		return false
	}

	// Don't update while actively proving
	if psState.IsProving {
		return false
	}

	// Don't update if in challenge window but haven't proven yet
	if psState.InChallengeWindow && !psState.HasProven {
		return false
	}

	// Otherwise it's safe
	return true
}
