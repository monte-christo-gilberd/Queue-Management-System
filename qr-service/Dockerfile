# Build
FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/qr-service ./cmd/qr-service

# Runtime
FROM alpine:3.20

RUN apk add --no-cache ca-certificates \
    && adduser -D -u 10001 app

COPY --from=build /out/qr-service /usr/local/bin/qr-service

USER app
EXPOSE 8090

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8090/health || exit 1

ENTRYPOINT ["/usr/local/bin/qr-service"]
