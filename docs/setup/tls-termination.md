# TLS Termination

Piri servers (both PDP and UCAN) do not handle TLS termination directly. For production deployments, you must use a reverse proxy to handle HTTPS connections and route traffic from your domains to the appropriate Piri servers.

## Prerequisites

Before configuring TLS, ensure you have:
- ✅ [Completed system prerequisites](./prerequisites.md) (including domain setup)
- ✅ [Installed Piri](./installation.md)
- ✅ [Generated cryptographic keys](./key-generation.md)

## Overview

This section configures how your domains (from the [Network Requirements](./prerequisites.md#network-requirements)) connect to your Piri servers:

```
Internet → Your Domain → Nginx (HTTPS) → Piri Server (HTTP)
         ↓                   ↓                ↓
   piri.example.com      Port 443        Port 3000 (UCAN)
   up.piri.example.com   Port 443        Port 3001 (PDP)
```

## Why TLS Termination is Required

- **Security**: Encrypts data in transit between clients and your server
- **Trust**: Required for browser connections and API integrations
- **Network Requirements**: Storacha Network requires HTTPS endpoints
- **Certificate Management**: Centralized SSL certificate handling

## DNS Configuration

Before proceeding, ensure your domains point to your server:

1. Configure DNS A records for both domains to point to your server's IP address:
   - `piri.example.com` → Your server IP
   - `up.piri.example.com` → Your server IP

2. Verify DNS propagation:
   ```bash
   dig piri.example.com
   dig up.piri.example.com
   ```

## Setting up Nginx

Nginx acts as a reverse proxy, accepting HTTPS connections on your domains and forwarding them to the appropriate Piri servers running locally.

### Prerequisites

```bash
# Install Nginx and Certbot
sudo apt update
sudo apt install -y nginx certbot python3-certbot-nginx
```

### Configuration Steps

**Step 1: Create Configuration Files**

Create separate configuration files for each domain:
- `/etc/nginx/sites-available/piri.example.com` (for UCAN server)
- `/etc/nginx/sites-available/up.piri.example.com` (for PDP server)

**Step 2: Configure UCAN Server (piri.example.com → Port 3000)**

Create `/etc/nginx/sites-available/piri.example.com`:

```nginx
server {
    server_name piri.example.com;  # Replace with your actual UCAN domain
    
    # For UCAN server handling client uploads
    client_max_body_size 0;           # Allow unlimited file uploads
    client_body_timeout 300s;         # Timeout for slow uploads
    client_header_timeout 300s;       # Timeout for slow connections
    send_timeout 300s;                # Timeout for sending responses
    
    location / {
        proxy_pass http://localhost:3000;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_cache_bypass $http_upgrade;
        
        proxy_request_buffering off; # Stream uploads directly to backend
    }
}
```

**Step 3: Configure PDP Server (up.piri.example.com → Port 3001)**

Create `/etc/nginx/sites-available/up.piri.example.com`:

```nginx
server {
    server_name up.piri.example.com;  # Replace with your actual PDP domain
    
    # PDP server also handles large uploads
    client_max_body_size 0;           # Allow unlimited file uploads
    client_body_timeout 300s;         # Timeout for slow uploads
    client_header_timeout 300s;       # Timeout for slow connections
    send_timeout 300s;                # Timeout for sending responses
    
    location / {
        proxy_pass http://localhost:3001;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_cache_bypass $http_upgrade;
        
        proxy_request_buffering off; # Stream uploads directly to backend
    }
}
```

**Step 4: Enable the Sites**

Enable both nginx configurations:

```bash
# Enable UCAN server configuration
sudo ln -s /etc/nginx/sites-available/piri.example.com /etc/nginx/sites-enabled/

# Enable PDP server configuration  
sudo ln -s /etc/nginx/sites-available/up.piri.example.com /etc/nginx/sites-enabled/

# Test configuration
sudo nginx -t

# Reload nginx
sudo systemctl reload nginx
```

**Step 5: Obtain SSL Certificates**

Obtain SSL certificates for both domains:

```bash
# For UCAN server domain (replace with your actual domain)
sudo certbot --nginx -d piri.example.com

# For PDP server domain (replace with your actual domain)
sudo certbot --nginx -d up.piri.example.com
```

## Port Configuration

**Default ports:**
- **UCAN Server**: 3000 (configurable via `--port`)
- **PDP Server**: 3001 (configurable via `--port`)
- **HTTPS**: 443 (standard)
- **HTTP**: 80 (redirect to HTTPS)

## Testing Your Configuration

After setting up TLS termination, verify HTTPS connectivity for both domains:

```bash
# Test UCAN server domain
curl -I https://piri.example.com

# Test PDP server domain  
curl -I https://up.piri.example.com
```

Both should return HTTP status 502 (Bad Gateway) until the Piri servers are started.

---

## Next Steps

After configuring TLS termination, proceed to set up:
- [PDP Server](../guides/pdp-server.md)
- [UCAN Server](../guides/ucan-server.md)