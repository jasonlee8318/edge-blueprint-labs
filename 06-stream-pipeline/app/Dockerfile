# ============================================================
# edge-stream-pipeline 빌드
# 이 빌드 단계는 도커 데몬이 실행되는 실제 환경(정상 인터넷)에서
# `docker compose build`로 실행됩니다 — go mod tidy가 필요한
# 모든 의존성을 이 시점에 새로 받습니다.
# ============================================================
FROM golang:1.25-bookworm AS builder

WORKDIR /app
COPY go.mod ./
COPY main.go ./

RUN go mod tidy && \
    CGO_ENABLED=0 GOOS=linux go build -o edge-stream-pipeline .

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /app/edge-stream-pipeline .
COPY res ./res

EXPOSE 59750
ENTRYPOINT ["/app/edge-stream-pipeline"]
