package initalize

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"time"
	
	"github.com/storacha/piri/cmd/cliutil"
)

func createPiriUser() error {
	// Check if user exists
	if _, err := user.Lookup("piri"); err == nil {
		return nil // User already exists
	}

	// Create system user and group
	cmd := exec.Command("useradd",
		"--system",              // System user
		"--no-create-home",      // No home directory
		"--shell", "/bin/false", // No shell access
		"--comment", "Piri Storage Service",
		"piri")

	return cmd.Run()
}

func setPiriOwnership(path string) error {
	piriUser, err := user.Lookup("piri")
	if err != nil {
		return err
	}

	uid, _ := strconv.Atoi(piriUser.Uid)
	gid, _ := strconv.Atoi(piriUser.Gid)

	return os.Chown(path, uid, gid)
}

func GeneratePiriService(binaryPath, command string, stopTimeout time.Duration) string {
	return fmt.Sprintf(`[Unit]
Description=Piri Storage Node Service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=piri
Group=piri
WorkingDirectory=%s
ExecStart=%s %s
TimeoutStopSec=%s
KillMode=mixed
Restart=on-failure
RestartSec=10
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
`, cliutil.PiriSystemDir, binaryPath, command, stopTimeout)
}

func GeneratePiriUpdaterService(binaryPath, command string) string {
	return fmt.Sprintf(`[Unit]
Description=Piri Auto-Update Service
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
ExecStart=%s %s
StandardOutput=journal
StandardError=journal
`, binaryPath, command)
}

func GeneratePiriUpdaterTimer(onBootSec, onUnitActiveSec, randomizedDelaySec time.Duration) string {
	return fmt.Sprintf(`[Unit]
Description=Piri Auto-Update Timer
Requires=piri-updater.service

[Timer]
OnBootSec=%s
OnUnitActiveSec=%s
RandomizedDelaySec=%s
Persistent=true

[Install]
WantedBy=timers.target
`, onBootSec, onUnitActiveSec, randomizedDelaySec)
}
