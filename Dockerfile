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
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o podforge .

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binary and assets
COPY --from=builder /build/podforge .
COPY --from=builder /build/templates ./templates
COPY --from=builder /build/migrations ./migrations
COPY --from=builder /build/static ./static

# Data volume
VOLUME /app/data

ENV PODFORGE_PORT=8080
ENV PODFORGE_DATA_DIR=/app/data

EXPOSE 8080

ENTRYPOINT ["./podforge"]
