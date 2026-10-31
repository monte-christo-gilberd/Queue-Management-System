# Kontainer qr-service (Go). Build context: folder qr-service/
# Target: dev (go run, restart manual: docker compose restart qr-service) atau prod (default).
FROM golang:1.27-alpine AS base
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

# ---- dev ----
FROM base AS dev
COPY . .
ENV PORT=8090
EXPOSE 8090
CMD ["go", "run", "./cmd/qr-service"]

# ---- build ----
FROM base AS build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/qr-service ./cmd/qr-service

# ---- prod (target terakhir = default) ----
FROM alpine:3.20 AS prod

RUN apk add --no-cache ca-certificates \
    && adduser -D -u 10001 app

COPY --from=build /out/qr-service /usr/local/bin/qr-service

USER app
EXPOSE 8090

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8090/health || exit 1

ENTRYPOINT ["/usr/local/bin/qr-service"]
