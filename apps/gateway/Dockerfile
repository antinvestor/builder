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
COPY ./apps/gateway ./apps/gateway
COPY ./internal ./internal

# Build static binary
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath \
    -ldflags="-s -w \
        -X github.com/pitabwire/frame/version.Repository=${REPOSITORY} \
        -X github.com/pitabwire/frame/version.Version=${VERSION} \
        -X github.com/pitabwire/frame/version.Commit=${REVISION} \
        -X github.com/pitabwire/frame/version.Date=${BUILDTIME}" \
    -o /app/binary ./apps/gateway/cmd/main.go

# ---------- Final ----------
FROM cgr.dev/chainguard/static:latest

LABEL maintainer="Feature Service Team"
LABEL org.opencontainers.image.title="Feature Gateway Service"
LABEL org.opencontainers.image.description="HTTP API gateway for feature requests"

ARG VERSION
ARG REVISION
ARG BUILDTIME
ARG REPOSITORY

LABEL org.opencontainers.image.version=$VERSION
LABEL org.opencontainers.image.revision=$REVISION
LABEL org.opencontainers.image.created=$BUILDTIME
LABEL org.opencontainers.image.source=$REPOSITORY

USER 65532:65532

WORKDIR /

COPY --from=builder /app/binary /gateway

EXPOSE 8080

ENTRYPOINT ["/gateway"]
