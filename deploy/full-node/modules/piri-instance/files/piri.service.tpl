[Unit]
Description=Piri Full Server
After=network.target

[Service]
Type=simple
User=piri
Group=piri

WorkingDirectory=/etc/piri

# Extended timeout for init command which can take up to 5 minutes
TimeoutStartSec=360

ExecStartPre=/bin/bash -c '/usr/local/bin/piri init \
  --registrar-url="${registrar_url}" \
  --data-dir=/data/piri \
  --temp-dir=/tmp/piri \
  --key-file=/etc/piri/service.pem \
  --wallet-file=/etc/piri/wallet.hex \
  --lotus-endpoint="${lotus_endpoint}" \
  --operator-email="${operator_email}" \
  --public-url="${public_url}" \
  > /etc/piri/config.toml'

ExecStart=/usr/local/bin/piri serve --config=/etc/piri/config.toml

Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target