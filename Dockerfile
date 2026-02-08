# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /build

# Dependencies first (cached layer)
COPY go.mod ./
COPY go.sum* ./
RUN go mod download

# Copy source
COPY . .

# Build static binary
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o skald .

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binary and assets
COPY --from=builder /build/skald .
COPY --from=builder /build/templates ./templates
COPY --from=builder /build/migrations ./migrations
COPY --from=builder /build/static ./static

# Data volume
VOLUME /app/data

ENV SKALD_PORT=7707
ENV SKALD_DATA_DIR=/app/data

EXPOSE 7707

ENTRYPOINT ["./skald"]
