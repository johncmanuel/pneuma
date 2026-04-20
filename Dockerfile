# this is for building the server (cmd/server/)

FROM --platform=$BUILDPLATFORM oven/bun:alpine AS ui-builder

WORKDIR /src

COPY package.json bun.lock tsconfig*.json ./
COPY dashboard/package.json ./dashboard/
COPY web/package.json ./web/
COPY frontend/package.json ./frontend/
COPY landing/package.json ./landing/
COPY packages/ ./packages/
COPY scripts/ ./scripts/
COPY assets/ ./assets/

RUN bun install --filter "!frontend" --filter "!landing" --frozen-lockfile

COPY dashboard/ ./dashboard/
RUN cd dashboard && bun run build

COPY web/ ./web/
RUN cd web && bun run build

# Native Cross Compilation
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

ARG TARGETOS
ARG TARGETARCH
ARG EMBED_UI=true

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

# get rest of backend code
COPY . .

COPY --from=ui-builder /src/dashboard/dist ./dashboard/dist
COPY --from=ui-builder /src/web/dist ./web/dist

RUN if [ "$EMBED_UI" = "true" ]; then \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /bin/prod/pneuma-server ./cmd/server ; \
    else \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -tags no_embed -trimpath -ldflags="-s -w" -o /bin/prod/pneuma-server ./cmd/server ; \
    fi


# Emulated (if arch differs from runner) but minimal
FROM alpine:latest

# fpcalc won't be included, but inevitably it will, maybe.
RUN apk add --no-cache \
    ca-certificates \
    ffmpeg

RUN adduser -D -s /bin/sh pneuma

WORKDIR /app

COPY --from=builder /bin/prod/pneuma-server /usr/local/bin/pneuma-server

ENV PNEUMA_SERVER_HOST=0.0.0.0
ENV PNEUMA_DATA_DIR=/data
RUN mkdir -p /data && chown -R pneuma:pneuma /data
VOLUME ["/data"]

USER pneuma

EXPOSE 8989 

ENTRYPOINT ["pneuma-server"]
