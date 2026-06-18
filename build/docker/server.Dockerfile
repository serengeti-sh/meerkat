ARG VERSION=0.0.1
ARG GIT_COMMIT=unknown

############################
# 1. Build Stage
############################
FROM golang:1.26.4-alpine3.23 AS builder

RUN apk add --no-cache \
      git \
      gcc \
      musl-dev \
      nodejs \
      npm

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

RUN npx @apidevtools/swagger-cli bundle api/openapi.yaml --outfile api/openapi.bundled.json --type json

ARG TARGETOS
ARG TARGETARCH
ARG VERSION
ARG GIT_COMMIT

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build \
    -ldflags="-w -s \
              -X main.version=${VERSION}" \
    -o meerkat-server ./cmd/meerkat-server

############################
# 2. Runtime Stage
############################
FROM alpine:3.24.0 AS runtime

ARG VERSION

LABEL org.opencontainers.image.title="Meerkat Server" \
      org.opencontainers.image.description="AI-powered observability agent" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.source="https://github.com/serengeti-sh/meerkat"

RUN apk add --no-cache \
      ca-certificates \
      tzdata \
      curl && \
    adduser -D -u 1001 meerkat

WORKDIR /app

COPY --from=builder /build/meerkat-server /app/meerkat-server
COPY --from=builder /build/api/openapi.bundled.json /app/api/openapi.bundled.json
COPY --from=builder /build/internal/tool/schemas /app/internal/tool/schemas

RUN mkdir -p /app/config && chown -R meerkat:meerkat /app

USER meerkat

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/v1/health || exit 1

ENV PORT=8080 \
    HTTP_OPENAPI_PATH=/app/api/openapi.bundled.json

ENTRYPOINT ["/app/meerkat-server"]
CMD ["analyzer", "serve"]
