# Build stage: compiles the binary with the exact Go version go.mod pins,
# regardless of what a hosting platform's own builder happens to support.
FROM golang:1.26-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /cronfetch ./cmd/api

# Runtime stage: just the binary, not the Go toolchain or source.
FROM alpine:3.20
# Needed so the binary can verify TLS certificates when it calls career
# sites and the Telegram API over https.
RUN apk add --no-cache ca-certificates

COPY --from=builder /cronfetch /cronfetch

EXPOSE 8080
ENTRYPOINT ["/cronfetch"]
