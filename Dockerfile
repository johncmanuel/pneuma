# this is for building the server (cmd/server/)

FROM --platform=$BUILDPLATFORM node:20-alpine AS ui-builder

WORKDIR /src

COPY dashboard/package*.json ./dashboard/
RUN cd dashboard && npm ci

COPY web/package*.json ./web/
RUN cd web && npm ci

COPY dashboard/ ./dashboard/
RUN cd dashboard && npm run build

COPY web/ ./web/
RUN cd web && npm run build

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
