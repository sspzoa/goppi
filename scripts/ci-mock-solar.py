#!/usr/bin/env python3
"""Minimal Solar chat SSE mock for CI headless smoke."""

from __future__ import annotations

import os
import socket
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


class Handler(BaseHTTPRequestHandler):
    def do_POST(self) -> None:
        if self.path != "/chat/completions":
            self.send_error(404)
            return
        length = int(self.headers.get("Content-Length", "0"))
        if length:
            self.rfile.read(length)
        body = (
            'data: {"choices":[{"delta":{"content":"ok"},"finish_reason":"stop"}]}\n\n'
            "data: [DONE]\n\n"
        )
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.end_headers()
        self.wfile.write(body.encode())

    def log_message(self, fmt: str, *args: object) -> None:
        pass


def main() -> None:
    host = "127.0.0.1"
    port = int(os.environ.get("MOCK_SOLAR_PORT", "19876"))
    sock = socket.socket()
    try:
        sock.bind((host, port))
    except OSError:
        sock.bind((host, 0))
        port = sock.getsockname()[1]
    sock.close()
    server = ThreadingHTTPServer((host, port), Handler)
    print(f"mock solar on http://{host}:{port}", flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
