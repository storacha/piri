[Unit]
Description=Piri Full Server
After=network.target%{ if needs_docker } docker.service%{ endif }
%{ if needs_docker ~}
Requires=docker.service
%{ endif ~}

[Service]
Type=simple
User=piri
Group=piri

WorkingDirectory=/etc/piri

# Extended timeout for init command which can take up to 5 minutes
TimeoutStartSec=360

ExecStartPre=/bin/bash -c '/usr/local/bin/piri init \
  --network="${network}" \
  --data-dir=/data/piri \
  --temp-dir=/tmp/piri \
  --key-file=/etc/piri/service.pem \
  --wallet-file=/etc/piri/wallet.hex \
  --lotus-endpoint="${lotus_endpoint}" \
  --operator-email="${operator_email}" \
  --public-url="${public_url}" \
%{ if database_backend == "postgres" ~}
  --db-type=postgres \
  --db-postgres-url="${postgres_url}" \
  --db-postgres-max-open-conns=${postgres_max_open_conns} \
  --db-postgres-max-idle-conns=${postgres_max_idle_conns} \
  --db-postgres-conn-max-lifetime=${postgres_conn_max_lifetime} \
%{ endif ~}
%{ if storage_backend == "minio" ~}
  --s3-endpoint="${s3_endpoint}" \
  --s3-bucket-prefix="${s3_bucket_prefix}" \
  --s3-access-key-id="${s3_access_key_id}" \
  --s3-secret-access-key="${s3_secret_access_key}" \
  --s3-insecure \
%{ endif ~}
  > /etc/piri/config.toml'

ExecStart=/usr/local/bin/piri serve --config=/etc/piri/config.toml

Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
