# ============================================================
# Mock Cloud
# 이 컨테이너를 정지시키면 "네트워크 장애"가 재현됩니다.
# App Service의 HTTPSender가 전송에 실패 -> Store&Forward가 자동으로
# Postgres에 저장했다가, 이 컨테이너를 다시 켜면 RetryInterval마다
# 자동 재전송을 시도합니다.
# ============================================================
import base64
import gzip
import json
from http.server import BaseHTTPRequestHandler, HTTPServer

received_count = 0


class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        global received_count
        length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(length)

        try:
            # CompressWithGZIP은 base64로 인코딩된 gzip 바이트를 body로 보냅니다.
            decompressed = gzip.decompress(base64.b64decode(body))
            payload = json.loads(decompressed)
        except Exception:
            payload = body.decode(errors="replace")

        received_count += 1
        print(f"[CLOUD RECEIVE #{received_count}] "
              f"압축 크기={len(body)}bytes  내용={payload}")

        self.send_response(200)
        self.end_headers()

    def log_message(self, fmt, *args):
        pass  # 기본 접속 로그는 생략(위에서 직접 출력)


if __name__ == "__main__":
    print("Mock Cloud 대기 중 (0.0.0.0:8090/ingest)")
    print("이 컨테이너를 멈추면 네트워크 장애 상황이 재현됩니다:")
    print("  docker compose stop mock-cloud")
    print("  docker compose start mock-cloud")
    HTTPServer(("0.0.0.0", 8090), Handler).serve_forever()
