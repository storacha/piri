package update

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/minio/selfupdate"
	"github.com/spf13/cobra"
	"github.com/storacha/piri/cmd/cliutil"
	"github.com/storacha/piri/pkg/build"
	"github.com/storacha/piri/pkg/config"
	"github.com/storacha/piri/pkg/pdp/httpapi/client"
)

/*
This represents the ideal update condition
- a challenge has been issued.
- piri completed the challenge
- the next challenge window is in 30 mins

*/

/*
╔═══════════════════════════════════════════════════════════════╗
║                        PROOF SET STATE                        ║
╚═══════════════════════════════════════════════════════════════╝

Note: Timestamps are estimated based on current epoch alignment with system time (30-second epochs).

CONFIGURATION
─────────────────────────
  Proof Set ID:            566
  Proving Period:          60 epochs (30 minutes)
  Challenge Window:        30 epochs (15 minutes)
  Owners:                  0x7469B47e006D0660aB92AE560b27A1075EEcF97F
                           0x0000000000000000000000000000000000000000
  Initialized:             true

SYSTEM VIEW (Local Node)
────────────────────────
  Current Epoch:           3012692 (est. 2025-09-12 19:59:04)
  Next Challenge Epoch:    3012690 (est. 2025-09-12 19:58:04, 1 minutes ago)
  Previous Challenge:      3012660 (est. 2025-09-12 19:43:04, 16 minutes ago)

  Status:
    • Challenge Issued:    true
    • In Challenge Window: true (ends epoch 3012720 (est. 2025-09-12 20:13:04, in 14 minutes))
    • In Fault State:      false
    • Has Proven:          true
    • Is Proving:          false

CONTRACT STATE (On-Chain)
─────────────────────────
  Next Challenge Window:   3012750 (est. 2025-09-12 20:28:04, in 29 minutes)
  Next Challenge Epoch:    3012690 (est. 2025-09-12 19:58:04, 1 minutes ago)
  Max Proving Period:      60 epochs (30 minutes)
  Challenge Window:        0 epochs (0 seconds)
  Challenge Range:         772323072

  Fees:
    • Proof Fee:           114.67 nanoFIL
    • Buffered Fee:        344.02 nanoFIL
*/

var (
	InternalUpdateCmd = &cobra.Command{
		Use:    "update-internal",
		Args:   cobra.NoArgs,
		Hidden: true,
		RunE:   doUpdateInternal,
	}
)

func init() {
	UpdateCmd.SetOut(os.Stdout)
	UpdateCmd.SetErr(os.Stderr)
}

func doUpdateInternal(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	// only linux can do auto update, since the "auto" bits require service files
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return fmt.Errorf("internal update not supported on %s platform", runtime.GOOS)
	}

	// this command requires access to the binary executable path
	// it cannot prompt for sudo access since it's run in an automated manner
	// fail if we cannot modify binary path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}
	if needsElevatedPrivileges(execPath) {
		if !cliutil.IsRunningAsRoot() {
			return fmt.Errorf("internal update lacks permissions for %s", execPath)
		}
	}

	currentVersion := build.Version
	cmd.Printf("Current version: %s\n", currentVersion)

	// Check for latest release
	release, err := getLatestRelease(ctx)
	if err != nil {
		return fmt.Errorf("failed to get latest release: %w", err)
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	currentVersionClean := strings.Split(strings.TrimPrefix(currentVersion, "v"), "-")[0]

	cmd.Printf("Latest version: %s\n", latestVersion)

	if currentVersionClean == latestVersion {
		cmd.Println("Already running the latest version")
		return nil
	}

	// to update we must have access to the full server config
	// which should be in a file provided to piri, or in
	// a canonical place: /etc/piri/config.toml
	userCfg, err := config.Load[config.FullServerConfig]()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	appCfg, err := userCfg.ToAppConfig()
	if err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}

	api, err := client.NewFromConfig(config.Client{
		Identity: userCfg.Identity,
		API: config.API{
			Endpoint: fmt.Sprintf("http://%s:%d", userCfg.Server.Host, userCfg.Server.Port),
		},
	})
	if err != nil {
		return fmt.Errorf("creating api: %w", err)
	}

	psState, err := api.GetProofSetState(ctx, appCfg.UCANService.ProofSetID)
	if err != nil {
		return fmt.Errorf("failed to get proof set state for update: %w", err)
	}

	// TODO there is an important decision to make here, if the node is in
	// a faulted state (psState can tell us this) the node won't be proving
	// it might be in a challenge window, but it probably won't have proven it
	// hard to say what to do here without some feedback from review

	// if the node is in a faulted state, we can probably update
	// there is a non-zero chance this state could exist because
	// of a software bug, and this update might be the fix.
	// Or the operator made a mistake, and will get out soon....
	if !psState.IsInFaultState {
		// TODO a separate decision here is regarding the calibration network contract as it has a:
		//
		// Proving Period: 60 epochs (30 minutes)
		// Challenge Window: 30 epochs (15 minutes)
		//
		// so we are submitting a proof every 30 mins (check maths)
		// and a new challenge is issued every 15 mins (check maths)
		// so a 10min gap exists where the node has received a challenge
		// but cannot prove it yet.
		// getting the below conditions to evaluate to true will require either
		// luck, or frequent update checks.
		// need to give this more thought, "production" with a proof due every 24hours will be easier.

		// don't update while proving
		if psState.IsProving {
			cmd.Println("Piri is actively proving, reject update")
		}

		// if within an unproven challenge window don't update
		if psState.InChallengeWindow && !psState.HasProven {
			cmd.Println("Piri is within an unproven challenge window, reject update")
		}
	}

	// we're good to update!

	// Find the appropriate asset for this platform
	assetURL, err := findAssetURL(release)
	if err != nil {
		return fmt.Errorf("failed to find appropriate release asset: %w", err)
	}

	cmd.Printf("Downloading update from %s\n", assetURL)

	// Get the filename from the URL
	assetFileName := path.Base(assetURL)

	// Download and parse checksums
	cmd.Println("Fetching checksums...")
	checksum, err := getAssetChecksum(ctx, cmd, release, assetFileName)
	if err != nil {
		return fmt.Errorf("failed to get asset checksum, aborting update: %w", err)
	}

	// Download and verify the archive, then extract the binary
	newBinary, err := downloadAndVerifyBinary(ctx, cmd, assetURL, checksum, false)
	if err != nil {
		return fmt.Errorf("failed to download update: %w", err)
	}
	defer newBinary.Close()

	// Apply the update (no checksum verification here since we already verified the archive)
	cmd.Println("Applying update...")
	err = selfupdate.Apply(newBinary, selfupdate.Options{
		TargetPath:  execPath,
		OldSavePath: execPath + ".old",
	})
	if err != nil {
		if rerr := selfupdate.RollbackError(err); rerr != nil {
			return fmt.Errorf("failed to apply update and rollback: %w", rerr)
		}
		return fmt.Errorf("failed to apply update: %w", err)
	}

	cmd.Printf("Successfully updated to version %s\n", latestVersion)

	// we've downloaded and applied the update. We need to restart the process
	// for the update to take effect.

	if err := exec.Command("systemctl", "restart", "piri").Run(); err != nil {
		return fmt.Errorf("failed to restart piri after update: %w", err)
	}

	return nil
}
