# Kontainer backend (Go, Gin). Build context: folder backend/
# Build dua tahap: kompilasi di golang, lalu binary statis di alpine yang kecil.
FROM golang:1.27-alpine AS build
WORKDIR /src

# Unduh dependensi dulu agar layer cache tetap berguna saat kode berubah.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/qms ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 app

COPY --from=build /out/qms /usr/local/bin/qms

# Konfigurasi lewat environment saat run (tidak dibakar ke image):
#   DATABASE_URL, JWT_SECRET, ALLOWED_ORIGINS (opsional), PORT (default 8080).
ENV PORT=8080
USER app
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/qms"]
