# Kontainer mail-service (Go, tanpa dependensi pihak ketiga). Build context: folder mail-service/
FROM golang:1.27-alpine AS base
WORKDIR /src
COPY go.mod ./

FROM base AS dev
COPY . .
CMD ["go", "run", "./cmd/mail-service"]

FROM base AS build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/mail-service ./cmd/mail-service

FROM alpine:3.20 AS prod
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 app
COPY --from=build /out/mail-service /usr/local/bin/mail-service
USER app
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=6 \
    CMD wget -qO- http://127.0.0.1:${PORT:-8092}/health || exit 1
CMD ["mail-service"]
