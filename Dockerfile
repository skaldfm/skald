# CSS build stage
FROM node:22-alpine AS css
WORKDIR /build
COPY static/css/input.css static/css/input.css
COPY templates/ templates/
RUN npm install tailwindcss @tailwindcss/typography
RUN npx @tailwindcss/cli -i static/css/input.css -o static/css/style.css --minify

# Go build stage
FROM golang:1.24-alpine AS builder
WORKDIR /build

# Dependencies first (cached layer)
COPY go.mod ./
COPY go.sum* ./
RUN go mod download

# Copy source
COPY . .

# Copy built CSS into static dir
COPY --from=css /build/static/css/style.css static/css/style.css

# Build static binary
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=${VERSION}" -o skald .

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata su-exec

WORKDIR /app

# Copy binary and assets
COPY --from=builder /build/skald .
COPY --from=builder /build/templates ./templates
COPY --from=builder /build/migrations ./migrations
COPY --from=builder /build/static ./static

# Entrypoint handles PUID/PGID user mapping
COPY entrypoint.sh .

# Data volume
VOLUME /app/data

ENV SKALD_PORT=7707
ENV SKALD_DATA_DIR=/app/data
ENV PUID=1000
ENV PGID=1000

EXPOSE 7707

ENTRYPOINT ["./entrypoint.sh"]
