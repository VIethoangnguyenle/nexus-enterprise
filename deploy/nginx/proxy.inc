# Shared by every proxied location (included inside a location block).
# The client address is the one nginx resolved via real-ip.conf; X-Forwarded-For is REPLACED with
# it, never appended to, so a chain the client sent cannot reach the services. The services trust
# X-Forwarded-For from this container's fixed address only (AUTH_TRUSTED_PROXIES).
proxy_http_version 1.1;
proxy_set_header Host $host;
proxy_set_header X-Real-IP $remote_addr;
proxy_set_header X-Forwarded-For $remote_addr;
proxy_set_header X-Forwarded-Proto https;
proxy_set_header Connection "";
proxy_connect_timeout 5s;
proxy_read_timeout 60s;
proxy_send_timeout 60s;
