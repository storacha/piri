# TLS Termination

The Piri node does not handle TLS termination directly. For production deployments, you must use a reverse proxy to handle HTTPS connections and route traffic from your domain to the Piri server.

## Prerequisites

Before configuring TLS, ensure you have:
- ✅ [Completed system prerequisites](./prerequisites.md) (including domain setup)
- ✅ [Installed Piri](./installation.md)
- ✅ [Generated cryptographic keys](./key-generation.md)

## Overview

This section configures how your domain (from the [Network Requirements](./prerequisites.md#network-requirements)) connects to your Piri server:

```
Internet → Your Domain → Nginx (HTTPS) → Piri Server (HTTP)
         ↓                   ↓                ↓
   piri.example.com      Port 443        Port 3000
```

## Why TLS Termination is Required

- **Security**: Encrypts data in transit between clients and your server
- **Trust**: Required for browser connections and API integrations
- **Network Requirements**: Storacha Network requires HTTPS endpoints
- **Certificate Management**: Centralized SSL certificate handling

## DNS Configuration

Before proceeding, ensure your domain points to your server:

1. Configure a DNS A record for your domain to point to your server's IP address:
   - `piri.example.com` → Your server IP

2. Verify DNS propagation:
   ```bash
   dig piri.example.com
   ```

## Setting up Nginx

Nginx acts as a reverse proxy, accepting HTTPS connections on your domain and forwarding them to the Piri server running locally.

### Prerequisites

```bash
# Install Nginx and Certbot
sudo apt update
sudo apt install -y nginx certbot python3-certbot-nginx
```

### Configuration Steps

**Step 1: Create Configuration File**

Create a configuration file for your domain:
- `/etc/nginx/sites-available/piri.example.com`

**Step 2: Configure Piri Server (piri.example.com → Port 3000)**

Create `/etc/nginx/sites-available/piri.example.com`:

```nginx
server {
    server_name piri.example.com;  # Replace with your actual domain
    
    # Piri server handles client uploads and storage operations
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

**Step 3: Enable the Site**

Enable the nginx configuration:

```bash
# Enable Piri server configuration
sudo ln -s /etc/nginx/sites-available/piri.example.com /etc/nginx/sites-enabled/

# Test configuration
sudo nginx -t

# Reload nginx
sudo systemctl reload nginx
```

**Step 4: Obtain SSL Certificate**

Obtain an SSL certificate for your domain:

```bash
# Replace with your actual domain
sudo certbot --nginx -d piri.example.com
```

## Port Configuration

**Default ports:**
- **Piri Server**: 3000 (configurable via `--port` or config file)
- **HTTPS**: 443 (standard)
- **HTTP**: 80 (redirect to HTTPS)

## Testing Your Configuration

After setting up TLS termination, verify HTTPS connectivity for your domain:

```bash
# Test your domain
curl -I https://piri.example.com
```

This should return HTTP status 502 (Bad Gateway) until the Piri server is started.

---

## Next Steps

After configuring TLS termination, proceed to:
- [Setup Piri Node](../guides/piri-server.md)