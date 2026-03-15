# ── Stage 1: сборка ──────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Зависимости кэшируются отдельным слоем
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Только сервер; sqlite не нужен — CGO отключаем для статической линковки
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o inspection-app ./cmd/server

# ── Stage 2: runtime ─────────────────────────────────────────────────────────
FROM alpine:3.20

# ca-certificates — HTTPS к Яндекс Диску; tzdata — временные зоны
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/inspection-app .
COPY --from=builder /app/web ./web

# Директории для загружаемых файлов (перекрываются volume-маунтами из compose)
RUN mkdir -p web/static/uploads web/static/documents

EXPOSE 8080

CMD ["./inspection-app"]
