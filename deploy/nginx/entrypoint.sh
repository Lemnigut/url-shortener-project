#!/bin/sh
# Если реального сертификата ещё нет — создаём временный самоподписанный,
# чтобы nginx смог стартовать и отдавать ACME-challenge для certbot.

CERT_DIR=/etc/letsencrypt/live/strugalem.ru

if [ ! -f "$CERT_DIR/fullchain.pem" ]; then
    echo "SSL-сертификат не найден, создаю временный самоподписанный..."
    apk add --no-cache openssl > /dev/null 2>&1
    mkdir -p "$CERT_DIR"
    openssl req -x509 -nodes -days 7 -newkey rsa:2048 \
        -keyout "$CERT_DIR/privkey.pem" \
        -out "$CERT_DIR/fullchain.pem" \
        -subj "/CN=strugalem.ru"
fi

exec nginx -g "daemon off;"
