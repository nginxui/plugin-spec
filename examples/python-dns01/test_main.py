#!/usr/bin/env python3
"""Drives server/main.py over stdin/stdout exactly as a host would.

Zero dependencies beyond the standard library. Run with:

    python3 test_main.py
"""
import json
import subprocess
import sys
from pathlib import Path

SERVER = Path(__file__).parent / "server" / "main.py"


def send(proc, message):
    proc.stdin.write(json.dumps(message) + "\n")
    proc.stdin.flush()


def recv(proc):
    line = proc.stdout.readline()
    assert line, "the plugin closed stdout before replying"
    return json.loads(line)


def test_initialize(proc):
    send(proc, {
        "jsonrpc": "2.0", "id": 1, "method": "plugin.initialize",
        "params": {
            "host": {"version": "2.7.0", "os": "linux", "arch": "amd64", "locale": "en"},
            "settings": {},
            "permissions": ["network"],
        },
    })
    response = recv(proc)
    assert response["id"] == 1, response
    assert response["result"]["api_version"] == 1, response
    assert response["result"]["capabilities"] == ["dns01"], response

    # plugin.initialized is a notification: no reply is expected.
    send(proc, {"jsonrpc": "2.0", "method": "plugin.initialized"})


def test_ping(proc):
    send(proc, {"jsonrpc": "2.0", "id": 2, "method": "plugin.ping"})
    response = recv(proc)
    assert response == {"jsonrpc": "2.0", "id": 2, "result": {}}, response


def test_unknown_method(proc):
    send(proc, {"jsonrpc": "2.0", "id": 3, "method": "dns01.check"})
    response = recv(proc)
    assert response["id"] == 3, response
    assert response["error"]["code"] == -32601, response


def test_exit(proc):
    send(proc, {"jsonrpc": "2.0", "method": "plugin.exit"})
    assert proc.wait(timeout=5) == 0, "the plugin did not exit cleanly"


def main():
    proc = subprocess.Popen(
        [sys.executable, str(SERVER)],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        bufsize=1,
    )
    try:
        test_initialize(proc)
        test_ping(proc)
        test_unknown_method(proc)
        test_exit(proc)
    finally:
        if proc.poll() is None:
            proc.kill()
            proc.wait()

    print("OK: initialize, ping, unknown method (-32601) and exit all behaved as specified")


if __name__ == "__main__":
    main()
