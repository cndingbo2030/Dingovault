# Multi-stage SaaS API image (CLI server only — no Wails UI in container).
FROM golang:1.25-alpine AS builder
ARG VERSION=dev
RUN apk add --no-cache git ca-certificates
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags="-s -w -X github.com/cndingbo2030/dingovault/internal/version.String=${VERSION}" \
    -o /out/dingovault ./cmd/dingovault

FROM alpine:3.21
LABEL org.opencontainers.image.source="https://github.com/cndingbo2030/dingovault"
LABEL org.opencontainers.image.licenses="AGPL-3.0"
RUN apk add --no-cache ca-certificates tzdata \
	&& addgroup -g 10001 -S dingovault \
	&& adduser -u 10001 -S -G dingovault -h /var/lib/dingovault dingovault
USER dingovault:dingovault
WORKDIR /var/lib/dingovault
ENV DINGO_ENV=production
ENV DINGO_PORT=12030
# Required at runtime: -e DINGO_JWT_SECRET='your-unique-secret-at-least-16-chars'
EXPOSE 12030
VOLUME ["/data"]
COPY --from=builder /out/dingovault /usr/local/bin/dingovault
ENTRYPOINT ["/usr/local/bin/dingovault", "-server", "-db", "/data/dingovault_saas.db"]
