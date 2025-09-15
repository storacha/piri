package initalize

import (
	"fmt"

	"github.com/storacha/piri/cmd/cliutil"
)

func GeneratePiriService(serviceUser string) string {
	return fmt.Sprintf(
		`[Unit]
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
`,
		serviceUser,
		serviceUser,
		cliutil.PiriSystemDir,
		cliutil.PiriBinaryPath,
		cliutil.PiriServeCommand,
		cliutil.PiriServerShutdownTimeout+cliutil.PiriServerShutdownTimeout,
	)
}

func GeneratePiriUpdaterService(serviceUser string) string {
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
`, serviceUser, serviceUser, cliutil.PiriSystemDir, cliutil.PiriBinaryPath, cliutil.PiriUpdateCommand)
}

func GeneratePiriUpdaterTimer() string {
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
`, cliutil.PiriUpdateBootDuration, cliutil.PiriUpdateUnitActiveDuration, cliutil.PiriUpdateRandomizedDelayDuration)
}
