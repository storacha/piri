package testutil

import (
	"runtime"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/storacha/piri/pkg/testutil/localdev"
)

type Harness struct {
	Container *localdev.Container
	Operator  *Operator
	Chain     *AnvilClient
}

func NewHarness(t testing.TB) *Harness {
	if runtime.GOOS == "darwin" {
		t.Skip("Skipping: container tests not supported on macOS")
	}
	ctx := t.Context()
	container, err := localdev.Run(ctx,
		localdev.WithStartupTimeout(3*time.Minute), // Allow ample time for container startup
		localdev.WithEmbeddedState(),               // Use embedded state files for portability
	)
	if err != nil {
		t.Fatal(err)
	}

	// Register container cleanup FIRST (runs last due to LIFO order)
	t.Cleanup(func() {
		_ = container.Terminate(ctx)
	})

	ethClient, err := ethclient.Dial(container.RPCEndpoint)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ethClient.Close()
	})

	chainClient, err := NewAnvilClient(container.RPCEndpoint)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		chainClient.Close()
	})

	operator := NewOperator(
		t,
		ethClient,
		chainClient,
		container.Addresses,
		localdev.Accounts,
	)

	return &Harness{
		Container: container,
		Operator:  operator,
		Chain:     chainClient,
	}
}
