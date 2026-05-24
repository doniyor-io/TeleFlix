#!/bin/zsh

GREEN='\033[0m\033[32m'
BLUE='\033[0m\033[34m'
RED='\033[0m\033[31m'
NC='\033[0m' # No Color

ENV_FILE=".env"
BACKEND_PORT=""
FRONTEND_PORT=""
BOT_TOKEN=""

echo -e "${BLUE}[INFO] Automation script has started...${NC}"

if [ ! -f "$ENV_FILE" ]; then
    echo -e "${RED}[ERROR] .env file not found${NC}"
    exit 1
fi

BACKEND_PORT=$(grep -E "^PORT=" $ENV_FILE | cut -d '=' -f2 | tr -d '\r' | tr -d ' ')
if [ -z "$BACKEND_PORT" ]; then
    BACKEND_PORT=9090
    echo -e "${RED}[WARN] PORT not found in .env, defaulting to ${BACKEND_PORT}${NC}"
fi

FRONTEND_PORT=$(grep -E "^FRONTEND_PORT=" $ENV_FILE | cut -d '=' -f2 | tr -d '\r' | tr -d ' ')
if [ -z "$FRONTEND_PORT" ]; then
    FRONTEND_PORT=3000
fi

BOT_TOKEN=$(grep -E "^TELEGRAM_BOT_TOKEN=" $ENV_FILE | cut -d '=' -f2 | tr -d '\r' | tr -d ' ')
if [ -z "$BOT_TOKEN" ]; then
    echo -e "${RED}[ERROR] TELEGRAM_BOT_TOKEN was not found in .env or is empty!${NC}"
    exit 1
fi

if ! command -v ngrok &> /dev/null; then
    echo -e "${RED}[WARN] Ngrok server was not found, thus, started downloading...${NC}"
    if command -v apt &> /dev/null; then
        curl -s https://ngrok-agent.s3.amazonaws.com/ngrok.asc | sudo tee /etc/apt/trusted.gpg.dev/ngrok.asc >/dev/null
        echo "deb https://ngrok-agent.s3.amazonaws.com buster main" | sudo tee /etc/apt/sources.list.dev/ngrok.list >/dev/null
        sudo apt update && sudo apt install ngrok -y
    elif command -v brew &> /dev/null; then
        brew install ngrok/ngrok/ngrok
    else
        echo -e "${RED}[ERROR] Package Manager (apt/brew) was not found. You can install ngrok manually!${NC}"
        exit 1
    fi
else
    echo -e "${GREEN}[INFO] Ngrok is already installed on your machine!${NC}"
fi

# Fondagi eski jarayonlarni tozalash
pkill ngrok 2>/dev/null
sleep 1

echo -e "${GREEN}[INFO] Opening an Ngrok tunnel on backend port $BACKEND_PORT...${NC}"
ngrok http $BACKEND_PORT --request-header-add "ngrok-skip-browser-warning: true" > /dev/null 2>&1 &

echo -e "${GREEN}[INFO] Opening an Ngrok tunnel on frontend port $FRONTEND_PORT...${NC}"
ngrok http $FRONTEND_PORT --request-header-add "ngrok-skip-browser-warning: true" > /dev/null 2>&1 &

echo -e "${BLUE}[INFO] Waiting for Ngrok to generate the tunnel URL...${NC}"

# SENING ORIGINAL 10 MARTALIK TRY LOOPING:
MAX_ATTEMPTS=10
ATTEMPT=1
BACKEND_NGROK_URL=""
FRONTEND_NGROK_URL=""
EXISTING_FRONTEND_URL=$(grep -E "^FRONTEND_URL=" "$ENV_FILE" | cut -d '=' -f2- | tr -d '\r' | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')

get_ngrok_url_for_port() {
    local port="$1"
    python3 - "$port" <<'PY'
import json
import sys
import urllib.request

port = sys.argv[1]
try:
    with urllib.request.urlopen("http://localhost:4040/api/tunnels", timeout=2) as resp:
        payload = json.load(resp)
except Exception:
    sys.exit(0)

target = f"http://localhost:{port}"
for tunnel in payload.get("tunnels", []):
    if tunnel.get("proto") == "https" and tunnel.get("config", {}).get("addr") == target:
        print(tunnel.get("public_url", ""))
        break
PY
}

while [ $ATTEMPT -le $MAX_ATTEMPTS ]; do
    BACKEND_NGROK_URL=$(get_ngrok_url_for_port "$BACKEND_PORT")
    FRONTEND_NGROK_URL=$(get_ngrok_url_for_port "$FRONTEND_PORT")

    if [ ! -z "$BACKEND_NGROK_URL" ]; then
        break
    fi

    echo -e "${BLUE}[INFO] Attempt $ATTEMPT/$MAX_ATTEMPTS: Tunnel not ready yet, waiting...${NC}"
    sleep 1.5
    ATTEMPT=$((ATTEMPT+1))
done

if [ -z "$BACKEND_NGROK_URL" ]; then
    echo -e "${RED}[ERROR] Failed to obtain backend Ngrok link after $MAX_ATTEMPTS attempts.${NC}"
    echo -e "${RED}[HINT] Terminalda 'pkill ngrok' qilib qayta urining. Token ulanganini tekshiring!${NC}"
    exit 1
fi

echo -e "${GREEN}[SUCCESS] Backend Ngrok link: $BACKEND_NGROK_URL${NC}"

if [ -z "$FRONTEND_NGROK_URL" ]; then
    if [ ! -z "$EXISTING_FRONTEND_URL" ]; then
        FRONTEND_NGROK_URL="$EXISTING_FRONTEND_URL"
        echo -e "${RED}[WARN] Frontend tunnel could not be opened. Using existing FRONTEND_URL from .env: $FRONTEND_NGROK_URL${NC}"
    else
        FRONTEND_NGROK_URL="$BACKEND_NGROK_URL"
        echo -e "${RED}[WARN] Frontend tunnel could not be opened. Falling back FRONTEND_URL to backend URL.${NC}"
    fi
else
    echo -e "${GREEN}[SUCCESS] Frontend Ngrok link: $FRONTEND_NGROK_URL${NC}"
fi

# Xavfsiz atrof-muhit yozish funksiyasi
update_env_var() {
    local key="$1"
    local value="$2"
    if grep -q "^${key}=" "$ENV_FILE"; then
        if [[ "$OSTYPE" == "darwin"* ]]; then
            sed -i '' "s|^\(${key}=\).*|\1${value}|" "$ENV_FILE"
        else
            sed -i "s|^\(${key}=\).*|\1${value}|" "$ENV_FILE"
        fi
    else
        echo "${key}=${value}" >> "$ENV_FILE"
    fi
}

update_env_var "WEBHOOK_URL" "$BACKEND_NGROK_URL"
update_env_var "NGROK_URL" "$BACKEND_NGROK_URL"
update_env_var "FRONTEND_URL" "$FRONTEND_NGROK_URL"

echo -e "${GREEN}[SUCCESS] New Ngrok URLs have successfully been injected in .env!${NC}"

echo -e "${BLUE}[INFO] Rebuilding and restarting Docker Containers with the new URL...${NC}"
sudo docker compose down
sudo docker compose up --build -d

echo -e "${BLUE}[INFO] Waiting for backend health check through Ngrok...${NC}"
HEALTH_OK=0
for attempt in {1..15}; do
    if curl -fsS "${BACKEND_NGROK_URL}/health" > /dev/null 2>&1; then
        HEALTH_OK=1
        break
    fi
    echo -e "${BLUE}[INFO] Health attempt ${attempt}/15: backend not ready yet...${NC}"
    sleep 2
done

if [ "$HEALTH_OK" -ne 1 ]; then
    echo -e "${RED}[ERROR] Backend did not become reachable through Ngrok: ${BACKEND_NGROK_URL}/health${NC}"
    exit 1
fi

echo -e "${BLUE}[INFO] Telegram Webhook updating...${NC}"
WEBHOOK_RES=$(curl -s -X POST "https://api.telegram.org/bot${BOT_TOKEN}/setWebhook?url=${BACKEND_NGROK_URL}/webhook")

if [[ "$WEBHOOK_RES" == *"\"ok\":true"* ]]; then
    echo -e "${GREEN}[SUCCESS] Telegram Webhook has been successfully connected!${NC}"
else
    echo -e "${RED}[ERROR] Telegram Webhook failed to connect. Cause: $WEBHOOK_RES${NC}"
    exit 1
fi

echo -e "${GREEN}[FINISH] All is ready now! Backend: ${BACKEND_NGROK_URL} | Frontend: ${FRONTEND_NGROK_URL}${NC}"