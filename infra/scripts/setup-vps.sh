#!/bin/bash
# infra/scripts/setup-vps.sh
#
# One-time idempotent setup for a fresh Ubuntu 24.04 VPS.
# Run manually as root ONCE — not by CI.
#
# Usage:
#   ssh root@YOUR_VPS_IP
#   curl -sSL https://raw.githubusercontent.com/YOUR_ORG/realestate/main/infra/scripts/setup-vps.sh | bash
set -euo pipefail

echo "==> Installing Docker..."
apt-get update -y
apt-get install -y ca-certificates curl gnupg

install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg \
    | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
chmod a+r /etc/apt/keyrings/docker.gpg

echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
  https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" \
  | tee /etc/apt/sources.list.d/docker.list > /dev/null

apt-get update -y
apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin

echo "==> Installing Google Cloud CLI (for backup script gcloud storage commands)..."
apt-get install -y apt-transport-https
curl https://packages.cloud.google.com/apt/doc/apt-key.gpg \
    | gpg --dearmor -o /usr/share/keyrings/cloud.google.gpg
echo "deb [signed-by=/usr/share/keyrings/cloud.google.gpg] \
  https://packages.cloud.google.com/apt cloud-sdk main" \
  | tee /etc/apt/sources.list.d/google-cloud-sdk.list > /dev/null
apt-get update -y
apt-get install -y google-cloud-cli

echo "==> Creating non-root deploy user..."
useradd -m -s /bin/bash deploy || true
usermod -aG docker deploy

echo "==> Creating app directory..."
mkdir -p /opt/realestate/scripts
chown -R deploy:deploy /opt/realestate

echo "==> Configuring firewall..."
ufw allow 22/tcp
ufw allow 80/tcp
ufw allow 443/tcp
ufw --force enable

echo ""
echo "VPS setup complete."
echo ""
echo "Next steps:"
echo "  1. Copy .env to /opt/realestate/.env"
echo "  2. Copy infra/docker-compose.yml and infra/docker-compose.prod.yml to /opt/realestate/"
echo "  3. Copy infra/scripts/backup.sh to /opt/realestate/scripts/backup.sh"
echo "  4. chmod +x /opt/realestate/scripts/backup.sh"
echo "  5. Add SSH public key to /home/deploy/.ssh/authorized_keys"
echo "  6. Add backup cron: crontab -u deploy -e"
echo "     0 2 * * * /opt/realestate/scripts/backup.sh >> /var/log/realestate-backup.log 2>&1"
