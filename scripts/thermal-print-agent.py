#!/usr/bin/env python3
"""Local TechLane thermal print agent.

Listens on 127.0.0.1:9199 and sends raw ESC/POS bytes to the CUPS queue
(POS-80 by default). The ops web app posts receipt bytes here so Chrome never
turns the slip into a PDF (which prints as garbage on ESC/POS printers).

Start once per counter session:
  python3 scripts/thermal-print-agent.py
"""

from __future__ import annotations

import hashlib
import json
import os
import subprocess
import sys
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

HOST = "127.0.0.1"
PORT = int(os.environ.get("TECHLANE_PRINT_PORT", "9199"))
PRINTER = os.environ.get("TECHLANE_PRINTER", "POS-80")
ALLOW_ORIGIN = os.environ.get(
    "TECHLANE_PRINT_ORIGIN",
    "https://app.techlane.co.ke,http://127.0.0.1:5173,http://localhost:5173",
)
# Same ESC/POS body posted 2–3× (STK race) would reprint the whole slip.
_DEDUPE_LOCK = threading.Lock()
_RECENT_HASHES: dict[str, float] = {}
_DEDUPE_SECONDS = 12.0


def allowed_origin(origin: str | None) -> str | None:
    if not origin:
        return None
    allowed = {o.strip() for o in ALLOW_ORIGIN.split(",") if o.strip()}
    return origin if origin in allowed else None


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def log_message(self, fmt: str, *args) -> None:
        sys.stderr.write("[thermal-print] " + (fmt % args) + "\n")

    def _cors(self) -> None:
        origin = allowed_origin(self.headers.get("Origin"))
        if origin:
            self.send_header("Access-Control-Allow-Origin", origin)
            self.send_header("Vary", "Origin")
            self.send_header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
            self.send_header("Access-Control-Allow-Headers", "Content-Type")

    def do_OPTIONS(self) -> None:
        self.send_response(204)
        self._cors()
        self.send_header("Content-Length", "0")
        self.end_headers()

    def do_GET(self) -> None:
        if self.path.rstrip("/") == "/health":
            body = json.dumps({"ok": True, "printer": PRINTER}).encode()
            self.send_response(200)
            self._cors()
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
            return
        self.send_error(404)

    def do_POST(self) -> None:
        if self.path.rstrip("/") != "/print":
            self.send_error(404)
            return
        length = int(self.headers.get("Content-Length", "0"))
        if length <= 0 or length > 1_000_000:
            self.send_error(400, "invalid body")
            return
        data = self.rfile.read(length)
        digest = hashlib.sha256(data).hexdigest()
        now = time.monotonic()
        with _DEDUPE_LOCK:
            cutoff = now - _DEDUPE_SECONDS
            for h, ts in list(_RECENT_HASHES.items()):
                if ts < cutoff:
                    del _RECENT_HASHES[h]
            if digest in _RECENT_HASHES:
                body = json.dumps(
                    {"ok": True, "deduped": True, "printer": PRINTER}
                ).encode()
                self.send_response(200)
                self._cors()
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)
                self.log_message("deduped identical receipt (%s…)", digest[:12])
                return
            _RECENT_HASHES[digest] = now
        try:
            proc = subprocess.run(
                ["lp", "-d", PRINTER, "-o", "raw", "-t", "techlane-receipt"],
                input=data,
                capture_output=True,
                check=False,
            )
        except FileNotFoundError:
            with _DEDUPE_LOCK:
                _RECENT_HASHES.pop(digest, None)
            self.send_error(500, "lp not found")
            return
        if proc.returncode != 0:
            with _DEDUPE_LOCK:
                _RECENT_HASHES.pop(digest, None)
            err = (proc.stderr or proc.stdout or b"lp failed").decode("utf-8", "replace")
            body = json.dumps({"ok": False, "error": err.strip()}).encode()
            self.send_response(502)
            self._cors()
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
            return
        job = (proc.stdout or b"").decode("utf-8", "replace").strip()
        body = json.dumps({"ok": True, "job": job, "printer": PRINTER}).encode()
        self.send_response(200)
        self._cors()
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


def main() -> int:
    server = ThreadingHTTPServer((HOST, PORT), Handler)
    print(f"TechLane thermal print agent on http://{HOST}:{PORT} → {PRINTER}", flush=True)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\nstopped", flush=True)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
