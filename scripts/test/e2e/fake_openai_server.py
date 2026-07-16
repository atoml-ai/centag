#!/usr/bin/env python3
"""
Fake OpenAI-compatible server for offline E2E tests.

Endpoints:
  - GET /health
  - GET /v1/models
  - POST /v1/chat/completions
"""

from __future__ import annotations

import argparse
import json
import re
import time
from http import HTTPStatus
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


def _json_bytes(obj: dict) -> bytes:
    return json.dumps(obj, ensure_ascii=False).encode("utf-8")


def _extract_text(content) -> str:
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        parts = []
        for item in content:
            if isinstance(item, dict) and item.get("type") == "text":
                parts.append(str(item.get("text", "")))
        return "".join(parts)
    return ""


def _extract_marker(question: str) -> str:
    if not question:
        return ""
    patterns = [
        r"请严格回复[:：]\s*([A-Za-z0-9_\-\.]+)",
        r"只回复且完整回复[:：]\s*([A-Za-z0-9_\-\.]+)",
    ]
    for pat in patterns:
        m = re.search(pat, question)
        if m:
            return m.group(1).strip()
    return ""


def _build_answer(question: str) -> str:
    marker = _extract_marker(question)
    if marker:
        return marker
    return f"FAKE_E2E_RESPONSE::{question[:120]}"


class Handler(BaseHTTPRequestHandler):
    server_version = "FakeOpenAI/1.0"

    def _set_json_headers(self, code: int = 200):
        self.send_response(code)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.end_headers()

    def do_GET(self):
        if self.path == "/health":
            self._set_json_headers(HTTPStatus.OK)
            self.wfile.write(_json_bytes({"status": "ok"}))
            return

        if self.path == "/v1/models":
            self._set_json_headers(HTTPStatus.OK)
            self.wfile.write(
                _json_bytes(
                    {
                        "object": "list",
                        "data": [
                            {"id": "glm-5.2", "object": "model", "owned_by": "fake-openai"},
                            {"id": "glm-4-flash", "object": "model", "owned_by": "fake-openai"},
                        ],
                    }
                )
            )
            return

        self._set_json_headers(HTTPStatus.NOT_FOUND)
        self.wfile.write(_json_bytes({"error": "not found"}))

    def do_POST(self):
        if self.path != "/v1/chat/completions":
            self._set_json_headers(HTTPStatus.NOT_FOUND)
            self.wfile.write(_json_bytes({"error": "not found"}))
            return

        length = int(self.headers.get("Content-Length", "0"))
        payload = {}
        if length > 0:
            raw = self.rfile.read(length)
            try:
                payload = json.loads(raw.decode("utf-8"))
            except Exception:
                self._set_json_headers(HTTPStatus.BAD_REQUEST)
                self.wfile.write(_json_bytes({"error": "invalid json"}))
                return

        messages = payload.get("messages", [])
        question = ""
        if isinstance(messages, list) and messages:
            for msg in reversed(messages):
                if isinstance(msg, dict) and msg.get("role") == "user":
                    question = _extract_text(msg.get("content"))
                    break

        answer = _build_answer(question)
        model = payload.get("model") or "glm-5.2"
        now = int(time.time())
        response = {
            "id": f"chatcmpl-fake-{now}",
            "object": "chat.completion",
            "created": now,
            "model": model,
            "choices": [
                {
                    "index": 0,
                    "message": {"role": "assistant", "content": answer},
                    "finish_reason": "stop",
                }
            ],
            "usage": {
                "prompt_tokens": max(1, len(question) // 4),
                "completion_tokens": max(1, len(answer) // 4),
                "total_tokens": max(2, (len(question) + len(answer)) // 4),
            },
        }

        self._set_json_headers(HTTPStatus.OK)
        self.wfile.write(_json_bytes(response))

    def log_message(self, fmt, *args):
        # Keep e2e output clean.
        return


def main():
    parser = argparse.ArgumentParser(description="Run fake OpenAI HTTP server for offline E2E.")
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", type=int, default=28081)
    args = parser.parse_args()

    srv = ThreadingHTTPServer((args.host, args.port), Handler)
    print(f"[fake-openai] listening on http://{args.host}:{args.port}")
    srv.serve_forever()


if __name__ == "__main__":
    main()

