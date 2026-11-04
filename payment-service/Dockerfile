# Kontainer payment-service (Go; pgx + stripe-go). Build context: folder payment-service/
FROM golang:1.27-alpine AS base
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

FROM base AS dev
COPY . .
CMD ["go", "run", "./cmd/payment-service"]

FROM base AS build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/payment-service ./cmd/payment-service

FROM alpine:3.20 AS prod
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 app
COPY --from=build /out/payment-service /usr/local/bin/payment-service
USER app
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=6 \
    CMD wget -qO- http://127.0.0.1:${PORT:-8093}/health || exit 1
CMD ["payment-service"]
