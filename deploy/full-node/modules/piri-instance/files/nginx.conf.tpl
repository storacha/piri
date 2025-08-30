server {
    server_name ${server_name};

    # Allow unlimited file upload size (0 = no limit)
    # Since this is a file upload service, we'll let the backend handle size restrictions
    client_max_body_size 0;

    # Increase timeout for receiving client request body (default is 60s)
    # Prevents timeout errors during slow uploads of large files
    client_body_timeout 300s;

    # Increase timeout for receiving client request headers (default is 60s)
    # Useful for clients with slow connections
    client_header_timeout 300s;

    # Increase timeout for transmitting response to client (default is 60s)
    # Prevents timeout when sending large responses back
    send_timeout 300s;

    # Buffer optimizations for AWS
    client_body_buffer_size 1M;
    proxy_buffers 32 128k;
    proxy_buffer_size 256k;
    proxy_busy_buffers_size 512k;
    proxy_temp_file_write_size 256k;
    proxy_max_temp_file_size 0;  # Disable temp files completely

    # TCP optimizations
    tcp_nodelay on;
    tcp_nopush on;

    location / {
        proxy_pass http://localhost:3000;
        proxy_http_version 1.1;
        proxy_request_buffering off;  # Critical for upload speed, stream directly to server

        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_cache_bypass $http_upgrade;

        # Proxy timeouts and buffering
        proxy_connect_timeout 300s;
        proxy_send_timeout 300s;
        proxy_read_timeout 300s;
        proxy_buffering off;  # Also disable response buffering
    }

    listen 80;
}

server {
    listen 80;
    server_name _;
    return 404;
}