# ── Stage 1: сборка React-фронтенда ──────────────────────────────────────────
FROM node:22-alpine AS frontend

WORKDIR /fe
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# ── Stage 2: сборка Go-бэкенда ───────────────────────────────────────────────
FROM golang:1.25-alpine AS backend

WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o inspection-app ./cmd/server

# ── Stage 3: runtime ─────────────────────────────────────────────────────────
FROM alpine:3.20

# ca-certificates — HTTPS к Яндекс Диску; tzdata — временные зоны
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=backend /src/inspection-app .
COPY --from=backend /src/web ./web
# Собранный SPA: main.go подхватывает его из ./web/spa автоматически
COPY --from=frontend /fe/dist ./web/spa

# Директории для загружаемых файлов (перекрываются volume-маунтами из compose)
RUN mkdir -p web/static/uploads web/static/documents

EXPOSE 8080

CMD ["./inspection-app"]
