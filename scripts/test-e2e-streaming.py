#!/usr/bin/env python3
"""E2E test script for AetherStream streaming features.

Usage:
    export AETHERSTREAM_AUTH_SECRET=your-secret-32-chars-long
    ./aetherstream &
    python3 scripts/test-e2e-streaming.py
"""

import subprocess
import json
import sys
import time

BASE_URL = "http://127.0.0.1:8081"
DLNA_URL = "http://127.0.0.1:8082"

def curl(method, path, data=None, headers=None, expect_code=None):
    cmd = ["curl", "-s", "-w", "\\n%{http_code}", "-X", method]
    if data:
        cmd += ["-H", "Content-Type: application/json", "-d", json.dumps(data)]
    if headers:
        for k, v in headers.items():
            cmd += ["-H", f"{k}: {v}"]
    cmd += [f"{BASE_URL}{path}"]
    result = subprocess.run(cmd, capture_output=True, text=True, timeout=10)
    lines = result.stdout.strip().split("\n")
    code = int(lines[-1])
    body = "\n".join(lines[:-1])
    if expect_code and code != expect_code:
        print(f"  FAIL: expected {expect_code}, got {code}: {body[:200]}")
        return None
    return {"code": code, "body": body}

def test(name, fn):
    print(f"\n[TEST] {name}")
    try:
        fn()
        print(f"  PASS")
        return True
    except AssertionError as e:
        print(f"  FAIL: {e}")
        return False
    except Exception as e:
        print(f"  ERROR: {e}")
        return False

# Global token
token = None

def test_register():
    global token
    r = curl("POST", "/auth/register", {"username": "testuser", "password": "testpass123"}, expect_code=201)
    assert r is not None, "Register failed"
    data = json.loads(r["body"])
    assert "id" in data
    print(f"  User created: {data['id'][:8]}...")

def test_login():
    global token
    r = curl("POST", "/auth/login", {"username": "testuser", "password": "testpass123"}, expect_code=200)
    assert r is not None
    data = json.loads(r["body"])
    token = data["token"]
    assert token
    print(f"  Token: {token[:30]}...")

def test_system_info():
    r = curl("GET", "/api/system/info", headers={"Authorization": f"Bearer {token}"}, expect_code=200)
    assert r is not None
    data = json.loads(r["body"])
    print(f"  Version: {data.get('version', 'N/A')}")

def test_libraries():
    r = curl("GET", "/api/libraries", headers={"Authorization": f"Bearer {token}"}, expect_code=200)
    assert r is not None
    data = json.loads(r["body"])
    print(f"  Libraries: {len(data)}")

def test_dlna_discovery():
    cmd = ["curl", "-s", f"{DLNA_URL}/device/description.xml"]
    result = subprocess.run(cmd, capture_output=True, text=True, timeout=5)
    assert result.returncode == 0
    assert "AetherStream" in result.stdout
    assert "AVTransport" in result.stdout
    print(f"  DLNA device description OK")

def test_dlna_avtransport():
    soap = '''<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <u:SetAVTransportURI xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
      <InstanceID>0</InstanceID>
      <CurrentURI>http://example.com/video.mp4</CurrentURI>
    </u:SetAVTransportURI>
  </s:Body>
</s:Envelope>'''
    cmd = [
        "curl", "-s", "-X", "POST",
        "-H", "Content-Type: text/xml; charset=utf-8",
        "-H", 'SOAPAction: "urn:schemas-upnp-org:service:AVTransport:1#SetAVTransportURI"',
        "-d", soap,
        f"{DLNA_URL}/AVTransport/control"
    ]
    result = subprocess.run(cmd, capture_output=True, text=True, timeout=5)
    assert result.returncode == 0
    assert "SetAVTransportURIResponse" in result.stdout
    print(f"  AVTransport SetAVTransportURI OK")

def test_websocket_playback():
    import websocket
    ws = websocket.create_connection(f"ws://127.0.0.1:8081/ws/playback?type=receiver")
    msg = json.loads(ws.recv())
    assert msg["type"] == "pair"
    device_id = msg["device_id"]
    print(f"  TV paired: {device_id}")
    ws.close()

def test_playback_api():
    r = curl("POST", "/api/playback/start", {"item_id": "test-item-1"}, headers={"Authorization": f"Bearer {token}"}, expect_code=201)
    assert r is not None
    data = json.loads(r["body"])
    session_id = data["session_id"]
    print(f"  Session: {session_id}")
    
    r2 = curl("POST", f"/api/playback/{session_id}/play", headers={"Authorization": f"Bearer {token}"}, expect_code=200)
    assert r2 is not None
    print(f"  Play command sent")

def test_static_app():
    r = curl("GET", "/app", expect_code=200)
    assert r is not None
    assert "doctype html" in r["body"].lower()
    print(f"  Web app served OK")

def test_tv_page():
    r = curl("GET", "/tv", expect_code=200)
    assert r is not None
    assert "doctype html" in r["body"].lower()
    print(f"  TV page served OK")

def main():
    print("=" * 50)
    print("AetherStream E2E Streaming Test")
    print("=" * 50)
    
    tests = [
        test_register,
        test_login,
        test_static_app,
        test_tv_page,
        test_system_info,
        test_libraries,
        test_dlna_discovery,
        test_dlna_avtransport,
        test_playback_api,
    ]
    
    passed = 0
    failed = 0
    
    for t in tests:
        if test(t.__name__, t):
            passed += 1
        else:
            failed += 1
    
    print("\n" + "=" * 50)
    print(f"Results: {passed} passed, {failed} failed")
    print("=" * 50)
    
    return 0 if failed == 0 else 1

if __name__ == "__main__":
    sys.exit(main())
