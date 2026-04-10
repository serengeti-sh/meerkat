#syntax=docker/dockerfile:1.4

ARG VERSION=0.0.1
ARG GIT_COMMIT=unknown

############################
# 1. Build Stage
############################
FROM golang:1.26.1-alpine3.23 AS builder

RUN apk add --no-cache \
      git \
      gcc \
      musl-dev \
      nodejs \
      npm

WORKDIR /build

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download && go mod verify

COPY . .

RUN npx @apidevtools/swagger-cli bundle api/openapi.yaml --outfile api/openapi.bundled.json --type json

ARG TARGETOS
ARG TARGETARCH
ARG VERSION
ARG GIT_COMMIT

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -a \
    -ldflags="-w -s \
              -X main.version=${VERSION}" \
    -o inspector-server ./cmd/inspector-server

############################
# 2. Runtime Stage
############################
FROM alpine:3.23.0 AS runtime

ARG VERSION

LABEL org.opencontainers.image.title="Inspector Server" \
      org.opencontainers.image.description="AI-powered observability agent" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.source="https://github.com/mandacode-labs/inspector"

RUN apk add --no-cache \
      ca-certificates \
      tzdata \
      curl && \
    adduser -D -u 1001 inspector

WORKDIR /app

COPY --from=builder /build/inspector-server /app/inspector-server
COPY --from=builder /build/api/openapi.bundled.json /app/api/openapi.bundled.json

RUN mkdir -p /app/config && chown -R inspector:inspector /app

USER inspector

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/v1/health || exit 1

ENV PORT=8080 \
    HTTP_OPENAPI_PATH=/app/api/openapi.bundled.json

ENTRYPOINT ["/app/inspector-server"]
CMD ["serve"]
