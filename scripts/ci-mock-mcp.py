#!/usr/bin/env python3
"""Minimal stdio MCP echo server for CI and cmd E2E tests."""

from __future__ import annotations

import json
import os
import sys


def read_frame() -> bytes:
    headers: dict[str, str] = {}
    while True:
        line = sys.stdin.buffer.readline()
        if line in (b"\r\n", b"\n", b""):
            break
        key, value = line.decode("ascii", "ignore").split(":", 1)
        headers[key.strip().lower()] = value.strip()
    length = int(headers.get("content-length", "0"))
    if length <= 0:
        return b""
    return sys.stdin.buffer.read(length)


def write_frame(payload: dict) -> None:
    raw = json.dumps(payload, separators=(",", ":")).encode()
    sys.stdout.buffer.write(f"Content-Length: {len(raw)}\r\n\r\n".encode())
    sys.stdout.buffer.write(raw)
    sys.stdout.buffer.flush()


def main() -> int:
    marker = os.environ.get("GOPPI_MCP_MARKER", "")
    while True:
        raw = read_frame()
        if not raw:
            return 0
        try:
            req = json.loads(raw)
        except json.JSONDecodeError:
            continue
        method = req.get("method", "")
        req_id = req.get("id")
        if method == "notifications/initialized":
            continue
        if method == "initialize":
            write_frame(
                {
                    "jsonrpc": "2.0",
                    "id": req_id,
                    "result": {
                        "protocolVersion": "2024-11-05",
                        "capabilities": {"tools": {}},
                        "serverInfo": {"name": "ci-mock-mcp"},
                    },
                }
            )
            continue
        if method == "tools/list":
            write_frame(
                {
                    "jsonrpc": "2.0",
                    "id": req_id,
                    "result": {
                        "tools": [
                            {
                                "name": "echo",
                                "description": "echo text",
                                "inputSchema": {
                                    "type": "object",
                                    "properties": {"text": {"type": "string"}},
                                },
                            }
                        ]
                    },
                }
            )
            continue
        if method == "tools/call":
            if marker:
                with open(marker, "w", encoding="utf-8") as fh:
                    fh.write("called")
            params = req.get("params") or {}
            text = "pong"
            args = params.get("arguments") or {}
            if isinstance(args, str):
                try:
                    args = json.loads(args)
                except json.JSONDecodeError:
                    args = {}
            if isinstance(args, dict) and args.get("text"):
                text = str(args["text"])
            write_frame(
                {
                    "jsonrpc": "2.0",
                    "id": req_id,
                    "result": {
                        "content": [{"type": "text", "text": text}],
                        "isError": False,
                    },
                }
            )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
