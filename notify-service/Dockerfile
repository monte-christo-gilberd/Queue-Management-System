# Kontainer notify-service (Go). Build context: folder notify-service/
FROM golang:1.25-alpine AS base
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

FROM base AS dev
COPY . .
CMD ["go", "run", "./cmd/notify-service"]

FROM base AS build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/notify-service ./cmd/notify-service

FROM alpine:3.20 AS prod
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 app
COPY --from=build /out/notify-service /usr/local/bin/notify-service
USER app
CMD ["notify-service"]
