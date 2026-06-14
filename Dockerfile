# ── Build stage ──────────────────────────────────────────────────────────────
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /sudopulse-connector ./cmd/connector

# ── Runtime stage ────────────────────────────────────────────────────────────
FROM alpine:3.19

RUN apk add --no-cache ca-certificates
RUN mkdir -p /etc/sudopulse-connector

COPY --from=builder /sudopulse-connector /usr/local/bin/

ENTRYPOINT ["sudopulse-connector"]
