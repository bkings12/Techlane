FROM node:20-bookworm-slim

WORKDIR /app

COPY package.json package-lock.json* ./
RUN npm ci --omit=dev

COPY server.js ./

# Persist sessions & chat cache via volumes (see deploy/docker-compose.whatsapp.yml)
VOLUME ["/app/auth_sessions", "/app/chat_data"]

ENV NODE_ENV=production
EXPOSE 3001

CMD ["node", "server.js"]
