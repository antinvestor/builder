ARG TARGETOS=linux
ARG TARGETARCH=amd64

# ---------- Builder ----------
FROM golang:1.22 AS builder

WORKDIR /app

ARG REPOSITORY
ARG VERSION=dev
ARG REVISION=none
ARG BUILDTIME

COPY go.mod go.sum ./
RUN go mod download

# Copy only what this service needs
COPY ./apps/executor ./apps/executor
COPY ./internal ./internal

# Build static binary
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath \
    -ldflags="-s -w \
        -X github.com/pitabwire/frame/version.Repository=${REPOSITORY} \
        -X github.com/pitabwire/frame/version.Version=${VERSION} \
        -X github.com/pitabwire/frame/version.Commit=${REVISION} \
        -X github.com/pitabwire/frame/version.Date=${BUILDTIME}" \
    -o /app/binary ./apps/executor/cmd/main.go

# ---------- Final ----------
# NOTE: Using alpine instead of static for Docker CLI access
FROM alpine:3.19

LABEL maintainer="Feature Service Team"
LABEL org.opencontainers.image.title="Feature Executor Service"
LABEL org.opencontainers.image.description="Sandboxed test execution service (requires Docker socket)"

ARG VERSION
ARG REVISION
ARG BUILDTIME
ARG REPOSITORY

LABEL org.opencontainers.image.version=$VERSION
LABEL org.opencontainers.image.revision=$REVISION
LABEL org.opencontainers.image.created=$BUILDTIME
LABEL org.opencontainers.image.source=$REPOSITORY

# Install Docker CLI for sandbox execution
RUN apk add --no-cache docker-cli ca-certificates

# Create non-root user with docker group access
RUN addgroup -g 1000 feature && \
    adduser -u 1000 -G feature -s /bin/sh -D feature && \
    addgroup feature docker 2>/dev/null || true

# Create workspace directory
RUN mkdir -p /var/lib/feature-service/workspaces && \
    chown -R feature:feature /var/lib/feature-service

WORKDIR /

COPY --from=builder /app/binary /executor

# Note: Run as root or with docker group to access Docker socket
# In production, use proper security policies
USER feature

EXPOSE 8080

ENTRYPOINT ["/executor"]
