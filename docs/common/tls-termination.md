# TLS Termination Setup

Piri servers (both PDP and UCAN) do not handle TLS termination directly. 
For production deployments, you must use a reverse proxy to handle HTTPS connections.

## Why TLS Termination is Required

- **Security**: Encrypts data in transit between clients and your server
- **Trust**: Required for browser connections and API integrations
- **Network Requirements**: Storacha Network requires HTTPS endpoints
- **Certificate Management**: Centralized SSL certificate handling

## Reverse Proxy Options

### Option 1: Nginx (Recommended)

Nginx is recommended due to its performance, reliability, and extensive documentation.

#### Prerequisites
```bash
# Install Nginx and Certbot
apt update
apt install -y nginx certbot python3-certbot-nginx
```

#### Configuration

1. Create configuration file: `/etc/nginx/sites-available/YOUR_UCAN_DOMAIN` and `/etc/nginx/sites-available/YOUR_PDP_DOMAIN`
2. Use the appropriate template:

**For UCAN Server (Port 3000):**
```nginx
server {
    server_name YOUR_UCAN_DOMAIN;
    
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

**For PDP Server (Port 3001):**
```nginx
server {
    server_name YOUR_PDP_DOMAIN;
    
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

3. Enable the site:
```bash
ln -s /etc/nginx/sites-available/YOUR_DOMAIN /etc/nginx/sites-enabled/
nginx -t
systemctl reload nginx
```

4. Obtain SSL certificate:
```bash
certbot --nginx -d YOUR_DOMAIN
```

### Option 2: Caddy

Caddy provides automatic HTTPS with minimal configuration.

#### Installation
```bash
apt install -y debian-keyring debian-archive-keyring apt-transport-https
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | tee /etc/apt/sources.list.d/caddy-stable.list
apt update
apt install caddy
```

#### Configuration

Create `/etc/caddy/Caddyfile`:

**For UCAN Server:**
```caddy
YOUR_UCAN_DOMAIN {
    reverse_proxy localhost:3000 {
        header_up X-Real-IP {remote_host}
        header_up X-Forwarded-For {remote_host}
        header_up X-Forwarded-Proto {scheme}
        
        # Handle large uploads
        request_body {
            max_size 0
        }
        
        # Timeouts for slow connections
        timeout {
            read 300s
            write 300s
        }
    }
}
```

**For PDP Server:**
```caddy
YOUR_PDP_DOMAIN {
    reverse_proxy localhost:3001 {
        header_up X-Real-IP {remote_host}
        header_up X-Forwarded-For {remote_host}
        header_up X-Forwarded-Proto {scheme}
        
        # Handle large uploads
        request_body {
            max_size 0
        }
        
        # Timeouts for slow connections
        timeout {
            read 300s
            write 300s
        }
    }
}
```

Start Caddy:
```bash
systemctl enable --now caddy
```

### Option 4: Direct HTTP (Development Only)

**⚠️ WARNING: Not suitable for production**

For local development, you can access Piri directly:
- UCAN Server: `http://localhost:3000`
- PDP Server: `http://localhost:3001`

**Risks of direct HTTP:**
- No encryption of data in transit
- Cannot integrate with Storacha network
- Browser security warnings
- No certificate validation

## Port Configuration

Default ports:
- UCAN Server: 3000 (configurable via `--port`)
- PDP Server: 3001 (configurable via `--port`)
- HTTPS: 443 (standard)
- HTTP: 80 (redirect to HTTPS)

## Testing Your Configuration

After setting up TLS termination:

1. Test HTTPS connectivity:
```bash
curl -I https://YOUR_DOMAIN
```

2. Check certificate:
```bash
openssl s_client -connect YOUR_DOMAIN:443 -servername YOUR_DOMAIN
```

## Next Steps

After configuring TLS termination:
- Test your HTTPS endpoint
- Update DNS records if needed
- Configure Piri to listen on correct local ports
- Proceed with service-specific setup