FROM golang:1.25-bookworm AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /pingpong ./cmd/pingpong/

FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        traceroute \
        iputils-ping \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /pingpong /usr/local/bin/pingpong

RUN mkdir -p /data

ENTRYPOINT ["pingpong"]
