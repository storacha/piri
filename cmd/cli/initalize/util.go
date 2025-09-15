package initalize

import (
	"fmt"
	"time"

	"github.com/storacha/piri/cmd/cliutil"
)

func GeneratePiriService(binaryPath, command, serviceUser string, stopTimeout time.Duration) string {
	return fmt.Sprintf(`[Unit]
Description=Piri Storage Node Service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=%s
Group=%s
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
`, serviceUser, serviceUser, cliutil.PiriSystemDir, binaryPath, command, stopTimeout)
}

func GeneratePiriUpdaterService(binaryPath, command, serviceUser string) string {
	return fmt.Sprintf(`[Unit]
Description=Piri Auto-Update Service
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
User=%s
Group=%s
WorkingDirectory=%s
ExecStart=%s %s
StandardOutput=journal
StandardError=journal
`, serviceUser, serviceUser, cliutil.PiriSystemDir, binaryPath, command)
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
