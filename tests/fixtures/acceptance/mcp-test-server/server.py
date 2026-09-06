#!/usr/bin/env python3
"""Minimal MCP stdio server for testing runtime verification.
Responds to initialize and tools/list requests via JSON-RPC over stdio.
"""
import sys
import json

def send_response(msg_id, result):
    res = {
        "jsonrpc": "2.0",
        "id": msg_id,
        "result": result
    }
    print(json.dumps(res), flush=True)

def main():
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        try:
            req = json.loads(line)
        except json.JSONDecodeError:
            continue

        method = req.get("method", "")
        msg_id = req.get("id")

        if method == "initialize":
            send_response(msg_id, {
                "protocolVersion": "2024-11-05",
                "capabilities": {"tools": {}},
                "serverInfo": {
                    "name": "test-mcp-server",
                    "version": "1.0.0"
                }
            })
        elif method == "tools/list":
            send_response(msg_id, {
                "tools": [
                    {"name": "get_status", "description": "Get server status", "inputSchema": {"type": "object"}}
                ]
            })
        elif method == "initialized":
            # Notification - no response needed
            pass
        else:
            if msg_id is not None:
                send_response(msg_id, {})

if __name__ == "__main__":
    main()
