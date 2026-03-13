# this is for building the server (cmd/server/)

FROM golang:1.24-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /bin/prod/pneuma-server ./cmd/server

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    ffmpeg \
    # fpcalc won't be included, but inevitably it will, maybe.
    && rm -rf /var/lib/apt/lists/*

RUN useradd -m -s /bin/bash pneuma

WORKDIR /app

COPY --from=builder /bin/prod/pneuma-server /usr/local/bin/pneuma-server

ENV PNEUMA_DATA_DIR=/data
RUN mkdir -p /data && chown -R pneuma:pneuma /data
VOLUME ["/data"]

USER pneuma

EXPOSE 8989 

ENTRYPOINT ["pneuma-server"]
