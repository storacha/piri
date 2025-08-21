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
cat > /etc/piri/service.pem <<'EOF'
${service_pem_content}
EOF

cat > /etc/piri/wallet.hex <<'EOF'
${wallet_hex_content}
EOF

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
certbot --nginx -d ${domain_name} --non-interactive --agree-tos --email forrest@storacha.network --redirect

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