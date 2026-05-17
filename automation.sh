#!/bin/zsh

GREEN='\033[0m\033[32m'
BLUE='\033[0m\033[34m'
RED='\033[0m\033[31m'
NC='\033[0m' # No Color

ENV_FILE=".env"
PORT=""
BOT_TOKEN=""

echo -e "${BLUE}[INFO] Automation script has started...${NC}"

if [ ! -f "$ENV_FILE" ]; then
    echo -e "${RED}[ERROR] .env file not found${NC}"
    exit 1
fi

PORT=$(grep -E "^PORT=" $ENV_FILE | cut -d '=' -f2 | tr -d '\r' | tr -d ' ')
if [ -z "$PORT" ]; then
    echo -e "${RED}[WARN] PORT not found in .env, defaulting to 9090${NC}"
fi

PORT=3000

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

pkill ngrok 2>/dev/null

echo -e "${GREEN}[INFO] Opening an Ngrok tunnel on port $PORT...${NC}"

ngrok http $PORT --request-header-add "ngrok-skip-browser-warning: true" > /dev/null 2>&1 &

echo -e "${BLUE}[INFO] Waiting for Ngrok to generate the tunnel URL...${NC}"

MAX_ATTEMPTS=10
ATTEMPT=1
NGROK_URL=""

while [ $ATTEMPT -le $MAX_ATTEMPTS ]; do
    NGROK_URL=$(curl -s http://localhost:4040/api/tunnels | grep -o 'https://[^"]*ngrok-free.app' | head -n 1)

    if [ ! -z "$NGROK_URL" ]; then
        break
    fi

    echo -e "${BLUE}[INFO] Attempt $ATTEMPT/$MAX_ATTEMPTS: Tunnel not ready yet, waiting...${NC}"
    sleep 1.5
    ATTEMPT=$((ATTEMPT+1))
done

if [ -z "$NGROK_URL" ]; then
    echo -e "${RED}[ERROR] Failed to obtain Ngrok link after $MAX_ATTEMPTS attempts.${NC}"
    echo -e "${RED}[HINT] Make sure you have set auth token via command: 'ngrok config add-authtoken <token>'${NC}"
    exit 1
fi

echo -e "${GREEN}[SUCCESS] New Ngrok link: $NGROK_URL${NC}"

if grep -q "^WEBHOOK_URL=" "$ENV_FILE"; then
    if [[ "$OSTYPE" == "darwin"* ]]; then
        sed -i '' "s|^\(WEBHOOK_URL=\).*|\1$NGROK_URL|" "$ENV_FILE"
        sed -i '' "s|^\(NGROK_URL=\).*|\1$NGROK_URL|" "$ENV_FILE" 2>/dev/null || echo "NGROK_URL=$NGROK_URL" >> "$ENV_FILE"
    else
        sed -i "s|^\(WEBHOOK_URL=\).*|\1$NGROK_URL|" "$ENV_FILE"
        sed -i "s|^\(NGROK_URL=\).*|\1$NGROK_URL|" "$ENV_FILE" 2>/dev/null || echo "NGROK_URL=$NGROK_URL" >> "$ENV_FILE"
    fi
else
    echo "WEBHOOK_URL=$NGROK_URL" >> "$ENV_FILE"
    echo "NGROK_URL=$NGROK_URL" >> "$ENV_FILE"
fi
echo -e "${GREEN}[SUCCESS] New Ngrok URL has successfully been injected in .env!${NC}"

echo -e "${BLUE}[INFO] Telegram Webhook updating...${NC}"
WEBHOOK_RES=$(curl -s -X POST "https://api.telegram.org/bot${BOT_TOKEN}/setWebhook?url=${NGROK_URL}/webhook")

if [[ "$WEBHOOK_RES" == *"\"ok\":true"* ]]; then
    echo -e "${GREEN}[SUCCESS] Telegram Webhook has been successfully connected! Description: $WEBHOOK_RES${NC}"
else
    echo -e "${RED}[ERROR] Telegram Webhook failed to connect. Possible cause: $WEBHOOK_RES${NC}"
    exit 1
fi

echo -e "${BLUE}[INFO] Rebuilding and restarting Docker Containers with the new URL...${NC}"
sudo docker compose down
sudo docker compose up --build -d

echo -e "${GREEN}[FINISH] All is ready now! Your infrastructure is fully live on port $PORT${NC}"