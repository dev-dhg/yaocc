ARG YAOCC_BASE_IMAGE=alpine:latest
ARG YAOCC_DOCKER_APK_PACKAGES=""
ARG YAOCC_DOCKER_RUN_COMMANDS=""

# Build Stage
FROM golang:alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build binaries
# Building to /app/build to match project structure, though inside container path can be anything
RUN go build -o /app/build/yaocc-server ./cmd/yaocc-server && \
    go build -o /app/build/yaocc ./cmd/yaocc

# Final Stage
FROM ${YAOCC_BASE_IMAGE}

WORKDIR /app

# Install runtime dependencies if needed (e.g., ca-certificates for HTTPS)
RUN apk add --no-cache ca-certificates tzdata bash
RUN if [ -n "$YAOCC_DOCKER_APK_PACKAGES" ]; then \
    apk add --no-cache $YAOCC_DOCKER_APK_PACKAGES; \
    fi

RUN if [ -n "$YAOCC_DOCKER_RUN_COMMANDS" ]; then \
    /bin/bash -c "$YAOCC_DOCKER_RUN_COMMANDS"; \
    fi

# Copy binaries from builder
COPY --from=builder /app/build/yaocc-server /usr/local/bin/yaocc-server
COPY --from=builder /app/build/yaocc /usr/local/bin/yaocc

# Set environment variable defaults
ENV YAOCC_CONFIG_DIR=/app/data
ENV SERVER_TOKEN=
ENV TELEGRAM_BOT_TOKEN=
ENV LOG=false
ENV LOG_FILE=agent.log

# Expose server port
EXPOSE 8080

# Create data directory
RUN mkdir -p /app/data

ARG YAOCC_USER=root

# Set ownership of the application directory
RUN chown -R $YAOCC_USER:$YAOCC_USER /app

# Switch to specified user
USER $YAOCC_USER

# Default command with conditional logic for logging
CMD sh -c 'if [ "$LOG" = "true" ]; then exec yaocc-server -level verbose -file "$LOG_FILE"; else exec yaocc-server; fi'
