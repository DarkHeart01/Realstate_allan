# backend/Dockerfile
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates curl

# Install golang-migrate so it's available in the final image
RUN curl -sSL https://github.com/golang-migrate/migrate/releases/download/v4.17.1/migrate.linux-amd64.tar.gz \
    | tar -xz -C /usr/local/bin migrate

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG BUILD_SHA=unknown

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o /app/api ./cmd/api

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o /app/worker ./cmd/worker

# ── Runtime image ──────────────────────────────────────────────────────────────
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/api           /app/api
COPY --from=builder /app/worker        /app/worker
COPY --from=builder /usr/local/bin/migrate /usr/local/bin/golang-migrate
COPY migrations/                       /migrations/

ARG BUILD_SHA=unknown
ENV BUILD_SHA=${BUILD_SHA}

EXPOSE 8080

CMD ["/app/api"]
