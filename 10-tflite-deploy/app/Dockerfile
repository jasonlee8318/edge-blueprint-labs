# ============================================================
# edge-tflite-infer 빌드
#
# go-tflite는 TensorFlow Lite C API 공유 라이브러리(libtensorflowlite_c.so)가
# 필요합니다. Bazel로 TensorFlow 전체를 빌드하는 대신, go-tflite 프로젝트가
# GitHub Release에 올려둔 "buildkit"(사전 컴파일된 lib+헤더)을 받아 씁니다.
# 이 방식은 실제로 다운로드해 빌드·실행까지 확인했습니다.
# ============================================================
FROM golang:1.23-bookworm AS builder

ARG GO_TFLITE_VERSION=v1.0.7

# 사전 컴파일된 TensorFlow Lite C API + XNNPACK 델리게이트 라이브러리/헤더
ADD https://github.com/mattn/go-tflite/releases/download/${GO_TFLITE_VERSION}/go-tflite-buildkit-${GO_TFLITE_VERSION}.tar.gz /tmp/buildkit.tar.gz
RUN tar -C /usr/local -xzf /tmp/buildkit.tar.gz && rm /tmp/buildkit.tar.gz && ldconfig

WORKDIR /app
COPY go.mod ./
COPY main.go ./

ENV CGO_ENABLED=1
RUN go mod tidy && \
    go build -o edge-tflite-infer .

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /app/edge-tflite-infer .
# 런타임에도 동적 라이브러리가 필요하므로 함께 복사합니다.
COPY --from=builder /usr/local/lib /usr/local/lib
RUN ldconfig

COPY res ./res
COPY model ./model

EXPOSE 59752
ENTRYPOINT ["/app/edge-tflite-infer"]
