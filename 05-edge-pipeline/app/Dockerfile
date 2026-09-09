# ============================================================
# edge-sensor-classifier 빌드
# ============================================================
FROM golang:1.23-bookworm AS builder

WORKDIR /app
COPY go.mod ./
COPY main.go ./

RUN go mod tidy && \
    CGO_ENABLED=0 GOOS=linux go build -o edge-sensor-classifier .

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /app/edge-sensor-classifier .
COPY res ./res

EXPOSE 59749
ENTRYPOINT ["/app/edge-sensor-classifier"]
