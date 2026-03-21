#!/bin/bash
set -euo pipefail

# Log all output
exec > >(tee -a /var/log/user-data.log)
exec 2>&1

echo "=== Starting Piri instance initialization ==="
echo "Domain: ${domain_name}"
echo "Install method: ${install_method}"
echo "Install source: ${install_source}"

# Update system
echo "=== Updating system packages ==="
apt-get update
apt-get upgrade -y

%{ if needs_docker ~}
# =============================================================================
# Install Docker for native backends
# =============================================================================
echo "=== Installing Docker ==="
apt-get install -y ca-certificates curl gnupg
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
chmod a+r /etc/apt/keyrings/docker.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null
apt-get update
apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
systemctl enable docker
systemctl start docker
%{ endif ~}

# Create piri user
echo "=== Creating piri user ==="
useradd -m -s /bin/bash piri

# Format and mount EBS volume
echo "=== Setting up EBS volume ==="
# For m6a instances, EBS volumes appear as NVMe devices
# Wait for the second NVMe device (first is root volume)
DEVICE=""
for i in {1..60}; do
    if [ -e /dev/nvme1n1 ]; then
        DEVICE="/dev/nvme1n1"
        echo "Found EBS volume at $DEVICE"
        break
    fi
    echo "Waiting for EBS volume to attach (attempt $i/60)..."
    sleep 5
done

if [ -z "$DEVICE" ]; then
    echo "ERROR: EBS volume not found after 5 minutes"
    exit 1
fi

# Check if already formatted
if ! blkid $DEVICE; then
    echo "Formatting EBS volume..."
    mkfs.ext4 $DEVICE
else
    echo "EBS volume already formatted"
fi

# Create mount point and mount
mkdir -p /data
mount $DEVICE /data

# Add to fstab for persistent mounting
UUID=$(blkid -s UUID -o value $DEVICE)
echo "UUID=$UUID /data ext4 defaults,nofail 0 2" >> /etc/fstab

# Create piri directories
echo "=== Creating piri directories ==="
mkdir -p /etc/piri
mkdir -p /data/piri
mkdir -p /tmp/piri

# Set ownership
chown -R piri:piri /etc/piri
chown -R piri:piri /data/piri
chown -R piri:piri /tmp/piri

%{ if needs_docker ~}
# =============================================================================
# Setup Docker Compose for backend services
# =============================================================================
echo "=== Setting up backend services ==="

# Create directories for container data
mkdir -p /data/postgres /data/minio

# Deploy docker-compose.yml
cat > /etc/piri/docker-compose.yml <<'DOCKEREOF'
${docker_compose}
DOCKEREOF

# Start containers
cd /etc/piri
docker compose up -d

# Wait for services to be healthy
echo "=== Waiting for backend services ==="
%{ if database_backend == "postgres" ~}
echo "Waiting for PostgreSQL..."
until docker compose exec -T piri-postgres pg_isready -U piri -d piri 2>/dev/null; do
  sleep 2
done
echo "PostgreSQL is ready"
%{ endif ~}
%{ if storage_backend == "minio" ~}
echo "Waiting for MinIO..."
until curl -sf http://localhost:9000/minio/health/live >/dev/null 2>&1; do
  sleep 2
done
echo "MinIO is ready"
%{ endif ~}
%{ endif ~}

# Install nginx
echo "=== Installing nginx ==="
apt-get install -y nginx

# Install certbot
echo "=== Installing certbot ==="
apt-get install -y snapd
snap install core
snap refresh core
snap install --classic certbot
ln -s /snap/bin/certbot /usr/bin/certbot

# Deploy sensitive files
echo "=== Deploying sensitive files ==="

# Handle service.pem - convert \n to actual newlines if needed
# This handles both formats: proper newlines and escaped \n from AWS Secrets Manager
printf '%b\n' '${service_pem_content}' > /etc/piri/service.pem

# Handle wallet.hex - no conversion needed as it's just hex
echo '${wallet_hex_content}' > /etc/piri/wallet.hex

# Set permissions on sensitive files
chmod 600 /etc/piri/service.pem
chmod 600 /etc/piri/wallet.hex
chown piri:piri /etc/piri/service.pem
chown piri:piri /etc/piri/wallet.hex

# Install Piri
echo "=== Installing Piri ==="
if [ "${install_method}" = "version" ]; then
    bash -c "$(cat <<'SCRIPT'
${install_from_release_script}
SCRIPT
)" -- "${install_source}"
else
    bash -c "$(cat <<'SCRIPT'
${install_from_branch_script}
SCRIPT
)" -- "${install_source}"
fi

# Deploy systemd service
echo "=== Deploying systemd service ==="
cat > /etc/systemd/system/piri.service <<'EOF'
${systemd_service_content}
EOF

# Configure nginx
echo "=== Configuring nginx ==="
cat > /etc/nginx/sites-available/piri <<'EOF'
${nginx_conf_content}
EOF

# Enable nginx site
ln -sf /etc/nginx/sites-available/piri /etc/nginx/sites-enabled/
rm -f /etc/nginx/sites-enabled/default

# Test nginx config
nginx -t

# Restart nginx
systemctl restart nginx

# Obtain SSL certificate
echo "=== Obtaining SSL certificate ==="
%{ if use_letsencrypt_staging ~}
echo "Using Let's Encrypt STAGING environment (certificates will be untrusted)"
certbot --nginx -d ${domain_name} --non-interactive --agree-tos --email ${operator_email} --redirect --staging
%{ else ~}
echo "Using Let's Encrypt PRODUCTION environment"
certbot --nginx -d ${domain_name} --non-interactive --agree-tos --email ${operator_email} --redirect
%{ endif ~}

# Enable and start piri service
echo "=== Starting Piri service ==="
systemctl daemon-reload
systemctl enable piri
systemctl start piri

# Check service status
sleep 5
systemctl status piri

echo "=== Piri instance initialization complete ==="
echo "Service available at: https://${domain_name}"