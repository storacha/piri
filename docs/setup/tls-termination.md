# TLS Termination

The Piri node does not handle TLS directly. For production use, you need a reverse proxy to handle secure HTTPS connections and send traffic from your domain to the Piri server.

## Prerequisites

Before setting up TLS, make sure you have:
- ✅ [Completed system prerequisites](./prerequisites.md) (including domain setup)
- ✅ [Downloaded Piri](./download.md)
- ✅ [Generated cryptographic keys](./key-generation.md)

## Overview

This section shows how your domain (from the [Network Requirements](./prerequisites.md#network-requirements)) connects to your Piri server:

```
Internet → Your Domain → Nginx (HTTPS) → Piri Server (HTTP)
         ↓                   ↓                ↓
   piri.example.com      Port 443        Port 3000
```

## Why You Need TLS

- **Security**: Protects data sent between clients and your server
- **Trust**: Needed for browsers and APIs to connect
- **Network Requirements**: Storacha Network needs HTTPS endpoints
- **Certificate Management**: Handles SSL certificates in one place

## DNS Configuration

Before you continue, make sure your domain points to your server:

1. Set up a DNS A record for your domain that points to your server's IP address:
   - `piri.example.com` → Your server IP

2. Check DNS is working:
   ```bash
   dig piri.example.com
   ```

## Setting up Nginx

Nginx works as a reverse proxy. It gets HTTPS connections on your domain and sends them to the Piri server running on your computer.

### Prerequisites

```bash
# Install Nginx and Certbot
sudo apt update
sudo apt install -y nginx certbot python3-certbot-nginx
```

### Setup Steps

**Step 1: Create Configuration File**

Make a configuration file for your domain:
- `/etc/nginx/sites-available/piri.example.com`

**Step 2: Set Up Piri Server (piri.example.com → Port 3000)**

Create the file `/etc/nginx/sites-available/piri.example.com`:

```nginx
server {
    server_name piri.example.com;  # Replace with your actual domain
    
    # Piri server handles file uploads and storage
    client_max_body_size 0;           # Allow any file size
    client_body_timeout 300s;         # Wait time for slow uploads
    client_header_timeout 300s;       # Wait time for slow connections
    send_timeout 300s;                # Wait time for sending responses
    
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
        
        proxy_request_buffering off; # Send uploads directly to Piri
    }
}
```

**Step 3: Enable the Site**

Turn on the nginx configuration:

```bash
# Enable Piri server configuration
sudo ln -s /etc/nginx/sites-available/piri.example.com /etc/nginx/sites-enabled/

# Test configuration
sudo nginx -t

# Reload nginx
sudo systemctl reload nginx
```

**Step 4: Obtain SSL Certificate**

Get an SSL certificate for your domain:

```bash
# Replace with your actual domain
sudo certbot --nginx -d piri.example.com
```

## Port Configuration

**Default ports:**
- **Piri Server**: 3000 (can change with `--port` or config file)
- **HTTPS**: 443 (standard secure port)
- **HTTP**: 80 (redirects to HTTPS)

## Testing Your Configuration

After setting up TLS, check that HTTPS works for your domain:

```bash
# Test your domain
curl -I https://piri.example.com
```

This should show HTTP status 502 (Bad Gateway) until you start the Piri server.

---

## Next Steps

After setting up TLS, continue to:
- [Initialize Your Piri Node](./initialization.md)