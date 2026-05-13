#syntax=docker/dockerfile:1.4

ARG VERSION=0.0.1
ARG GIT_COMMIT=unknown

############################
# 1. Build Stage
############################
FROM golang:1.26.1-alpine3.23 AS builder

RUN apk add --no-cache \
      git=2.52.0-r0 \
      gcc=15.2.0-r2 \
      musl-dev=1.2.5-r23 \
      nodejs=24.14.1-r0 \
      npm=11.11.0-r0

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
    -o meerkat-server ./cmd/meerkat-server

############################
# 2. Runtime Stage
############################
FROM alpine:3.23.0 AS runtime

ARG VERSION

LABEL org.opencontainers.image.title="Meerkat Server" \
      org.opencontainers.image.description="AI-powered observability agent" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.source="https://github.com/serengeti-sh/meerkat"

RUN apk add --no-cache \
      ca-certificates=20260413-r0 \
      tzdata=2026b-r0 \
      curl=8.17.0-r1 && \
    adduser -D -u 1001 meerkat

WORKDIR /app

COPY --from=builder /build/meerkat-server /app/meerkat-server
COPY --from=builder /build/api/openapi.bundled.json /app/api/openapi.bundled.json
COPY --from=builder /build/resources/schemas /app/resources/schemas

RUN mkdir -p /app/config && chown -R meerkat:meerkat /app

USER meerkat

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/v1/health || exit 1

ENV PORT=8080 \
    HTTP_OPENAPI_PATH=/app/api/openapi.bundled.json

ENTRYPOINT ["/app/meerkat-server"]
CMD ["serve"]
