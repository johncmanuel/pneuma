# this is for building the server (cmd/server/)

FROM golang:1.24-alpine AS builder

ARG EMBED_UI=true

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

# lazy way of copying lol
COPY . .

RUN if [ "$EMBED_UI" = "true" ]; then \
    apk add --no-cache nodejs npm && \
    cd dashboard && npm ci && npm run build && cd .. && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /bin/prod/pneuma-server ./cmd/server ; \
    else \
    CGO_ENABLED=0 go build -tags no_embed -trimpath -ldflags="-s -w" -o /bin/prod/pneuma-server ./cmd/server ; \
    fi

# FROM debian:bookworm-slim

# RUN apt-get update && apt-get install -y --no-install-recommends \
#     ca-certificates \
#     ffmpeg \
#     && rm -rf /var/lib/apt/lists/*

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
