[Unit]
Description=Piri Full Server
After=network.target

[Service]
Type=simple
User=piri
Group=piri

WorkingDirectory=/etc/piri

ExecStartPre=/bin/bash -c '/usr/local/bin/piri init \
  --registrar-url="${registrar_url}" \
  --data-dir=/data/piri \
  --temp-dir=/tmp/piri \
  --key-file=/etc/piri/service.pem \
  --wallet-file=/etc/piri/wallet.hex \
  --lotus-endpoint="${lotus_endpoint}" \
  --operator-email="${operator_email}" \
  --public-url="${public_url}" \
  > /etc/piri/config.toml 2>/var/log/piri-init.log'

ExecStart=/usr/local/bin/piri serve full --config=/etc/piri/config.toml

Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target