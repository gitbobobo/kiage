#!/usr/bin/env python3
"""Import WorkosCursorSessionToken from Microsoft Edge (macOS)."""
import json
import os
import sys

def main():
    try:
        import browser_cookie3
    except ImportError:
        print("install: pip install browser-cookie3", file=sys.stderr)
        return 1
    token = None
    for c in browser_cookie3.edge(domain_name="cursor.com"):
        if c.name == "WorkosCursorSessionToken":
            token = c.value
            break
    if not token:
        print("WorkosCursorSessionToken not found in Edge", file=sys.stderr)
        return 1
    root = os.environ.get("KIAGE_ROOT", ".")
    path = os.path.join(root, "etc", "config.json")
    cfg = {"version": 1, "timezone": "Asia/Shanghai", "refresh_interval_sec": 600, "cursor": {}}
    if os.path.exists(path):
        with open(path) as f:
            cfg = json.load(f)
    cfg.setdefault("cursor", {})["session_token"] = token
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w") as f:
        json.dump(cfg, f, indent=2)
        f.write("\n")
    print(f"ok token_len={len(token)}")
    return 0

if __name__ == "__main__":
    raise SystemExit(main())
