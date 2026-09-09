# ============================================================
# edge-store-forward 빌드
# App Functions SDK v4는 Go 1.25 이상을 요구합니다 (go.mod 참고).
# ============================================================
FROM golang:1.25-bookworm AS builder

WORKDIR /app
COPY go.mod ./
COPY main.go ./

RUN go mod tidy && \
    CGO_ENABLED=0 GOOS=linux go build -o edge-store-forward .

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /app/edge-store-forward .
COPY res ./res

EXPOSE 59751
ENTRYPOINT ["/app/edge-store-forward"]
