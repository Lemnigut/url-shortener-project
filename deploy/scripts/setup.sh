#!/usr/bin/env bash
# Скрипт первоначальной настройки Ubuntu-сервера для TinyURL.
# Запускать от root: bash setup.sh <домен> <email>
set -euo pipefail

DOMAIN="${1:-strugalem.ru}"
EMAIL="${2:-ylogachev-sfedu@mail.ru}"
DEPLOY_USER="deploy"
APP_DIR="/opt/tinyurl"

echo "=== 1. Обновление системы ==="
apt-get update && apt-get upgrade -y

echo "=== 2. Установка Docker ==="
apt-get install -y ca-certificates curl gnupg
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
chmod a+r /etc/apt/keyrings/docker.gpg

echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
  https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" \
  > /etc/apt/sources.list.d/docker.list

apt-get update
apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin

echo "=== 3. Создание пользователя $DEPLOY_USER ==="
if ! id "$DEPLOY_USER" &>/dev/null; then
    adduser --disabled-password --gecos "" "$DEPLOY_USER"
    usermod -aG docker "$DEPLOY_USER"
    echo "$DEPLOY_USER ALL=(ALL) NOPASSWD: ALL" > "/etc/sudoers.d/$DEPLOY_USER"
fi

echo "=== 4. Генерация SSH-ключа для GitHub Actions ==="
DEPLOY_SSH_DIR="/home/$DEPLOY_USER/.ssh"
mkdir -p "$DEPLOY_SSH_DIR"
if [ ! -f "$DEPLOY_SSH_DIR/id_ed25519" ]; then
    ssh-keygen -t ed25519 -f "$DEPLOY_SSH_DIR/id_ed25519" -N "" -C "github-actions-deploy"
    cat "$DEPLOY_SSH_DIR/id_ed25519.pub" >> "$DEPLOY_SSH_DIR/authorized_keys"
    chmod 600 "$DEPLOY_SSH_DIR/authorized_keys"
    chown -R "$DEPLOY_USER:$DEPLOY_USER" "$DEPLOY_SSH_DIR"

    echo ""
    echo ">>> Приватный ключ для GitHub Secret SSH_KEY:"
    echo "---"
    cat "$DEPLOY_SSH_DIR/id_ed25519"
    echo "---"
    echo ""
fi

echo "=== 5. Создание директории приложения ==="
mkdir -p "$APP_DIR"
chown "$DEPLOY_USER:$DEPLOY_USER" "$APP_DIR"

echo "=== 6. Установка certbot ==="
apt-get install -y certbot

echo "=== 7. Получение SSL-сертификата ==="
certbot certonly --standalone -d "$DOMAIN" --email "$EMAIL" --agree-tos --non-interactive

echo "=== 8. Настройка автообновления сертификатов ==="
systemctl enable certbot.timer
systemctl start certbot.timer

echo ""
echo "=== Готово! ==="
echo "Следующие шаги:"
echo "  1. Склонируйте репозиторий:  su - $DEPLOY_USER -c 'git clone <REPO_URL> $APP_DIR'"
echo "  2. Создайте .env:            cp $APP_DIR/deploy/docker/.env.example $APP_DIR/deploy/docker/.env"
echo "  3. Заполните .env реальными значениями"
echo "  4. Обновите домен в nginx.conf: deploy/nginx/nginx.conf"
echo "  5. Запустите:                 cd $APP_DIR && docker compose -f deploy/docker/docker-compose.prod.yml up -d"
echo ""
echo "Не забудьте добавить в GitHub Secrets:"
echo "  SSH_HOST, SSH_USER ($DEPLOY_USER), SSH_KEY (ключ выше), POSTGRES_PASSWORD"
