#!/usr/bin/env python3
"""
AetherStream Autonomous Orchestrator
Runs every hour via cron. Checks project state, runs SwarmForge audits,
and triggers next phase if conditions are met.
"""

import os
import sys
import json
import subprocess
import time
import urllib.request
from datetime import datetime

PROJECT_DIR = "/home/devuser/dev/aetherstream"
STATE_FILE = os.path.join(PROJECT_DIR, ".aetherstate.json")
LOG_FILE = os.path.join(PROJECT_DIR, ".aetherlog.txt")

# Telegram config from Hermès .env
TELEGRAM_BOT_TOKEN = "8257630459:AAFZfFtJX_TVS_dz5MIvpsRzZOaKyzOHm_0"
TELEGRAM_CHAT_ID = "1479175411"

PHASES = [
    "phase1_foundation",      # HTTP server, DB, auth, config
    "phase2_library_engine",    # Scanner, naming, library CRUD
    "phase3_streaming",         # FFmpeg, HLS, transcode manager
    "phase4_swiftflow",         # SwiftFlow integration, QoS
    "phase5_polish",            # WebSocket, metadata providers, tests
]

def send_telegram(message):
    """Send report to Telegram"""
    try:
        url = f"https://api.telegram.org/bot{TELEGRAM_BOT_TOKEN}/sendMessage"
        data = json.dumps({
            "chat_id": TELEGRAM_CHAT_ID,
            "text": message,
            "parse_mode": "Markdown",
        }).encode()
        req = urllib.request.Request(url, data=data, headers={"Content-Type": "application/json"})
        urllib.request.urlopen(req, timeout=15)
    except Exception as e:
        log(f"Telegram send failed: {e}")

def log(msg):
    ts = datetime.now().isoformat()
    line = f"[{ts}] {msg}\n"
    with open(LOG_FILE, "a") as f:
        f.write(line)
    print(line, end="")

def load_state():
    if os.path.exists(STATE_FILE):
        with open(STATE_FILE) as f:
            return json.load(f)
    return {
        "current_phase": 0,
        "phase_status": {p: "pending" for p in PHASES},
        "last_audit": None,
        "health_score": 0.0,
        "build_ok": False,
        "tests_ok": False,
        "last_run": None,
        "crashes": [],
    }

def save_state(state):
    with open(STATE_FILE, "w") as f:
        json.dump(state, f, indent=2)

def run_cmd(cmd, cwd=PROJECT_DIR, timeout=120):
    try:
        result = subprocess.run(
            cmd, shell=True, cwd=cwd,
            capture_output=True, text=True, timeout=timeout
        )
        return result.returncode, result.stdout, result.stderr
    except subprocess.TimeoutExpired:
        return -1, "", "TIMEOUT"
    except Exception as e:
        return -1, "", str(e)

def check_build():
    log("Checking build...")
    rc, out, err = run_cmd("go build ./...")
    ok = rc == 0
    if not ok:
        log(f"BUILD FAILED: {err[:500]}")
    return ok

def check_tests():
    log("Checking tests...")
    rc, out, err = run_cmd("go test ./...")
    ok = rc == 0
    if not ok:
        log(f"TESTS FAILED: {err[:500]}")
    return ok

def run_health_check():
    log("Running health check...")
    rc, out, err = run_cmd("""
        loc=$(find . -name '*.go' | xargs wc -l | tail -1 | awk '{print $1}')
        go_files=$(find . -name '*.go' | wc -l)
        test_files=$(find . -name '*_test.go' | wc -l)
        echo "loc:$loc files:$go_files tests:$test_files"
    """)
    log(f"Health metrics: {out.strip()}")
    try:
        parts = out.strip().split()
        loc = int(parts[0].split(":")[1])
        tests = int(parts[2].split(":")[1])
        score = min(10.0, (loc / 1000) * 0.3 + (tests * 0.5) + 2.0)
    except:
        score = 0.0
    log(f"Estimated health score: {score:.1f}/10")
    return score

def run_swarmforge_audit():
    log("Running SwarmForge audit...")
    rc, out, err = run_cmd("sf audit project")
    log(f"SF audit output: {out[:1000]}")
    return rc == 0

def detect_crash(state):
    last = state.get("last_run")
    if last is None:
        return False
    last_dt = datetime.fromisoformat(last)
    hours_since = (datetime.now() - last_dt).total_seconds() / 3600
    current = PHASES[state["current_phase"]]
    if hours_since > 2 and state["phase_status"].get(current) == "in_progress":
        return True
    return False

def advance_phase(state):
    current_idx = state["current_phase"]
    if current_idx >= len(PHASES) - 1:
        log("ALL PHASES COMPLETE")
        return
    current = PHASES[current_idx]
    next_p = PHASES[current_idx + 1]
    state["phase_status"][current] = "completed"
    state["current_phase"] = current_idx + 1
    state["phase_status"][next_p] = "in_progress"
    log(f"ADVANCED: {current} -> {next_p}")

def main():
    log("=" * 50)
    log("AetherStream Autonomous Orchestrator starting")
    os.chdir(PROJECT_DIR)
    state = load_state()
    state["last_run"] = datetime.now().isoformat()
    
    if detect_crash(state):
        crash_phase = PHASES[state["current_phase"]]
        log(f"CRASH DETECTED in {crash_phase}")
        state["crashes"].append({
            "phase": crash_phase,
            "time": datetime.now().isoformat(),
        })
        state["phase_status"][crash_phase] = "crashed"
        run_swarmforge_audit()
        state["phase_status"][crash_phase] = "pending"
        state["current_phase"] = max(0, state["current_phase"] - 1)
    
    build_ok = check_build()
    tests_ok = check_tests()
    state["build_ok"] = build_ok
    state["tests_ok"] = tests_ok
    
    if not build_ok:
        log("BUILD BROKEN -- halting phase advancement, running audit")
        run_swarmforge_audit()
        save_state(state)
        return
    
    score = run_health_check()
    state["health_score"] = score
    
    current_idx = state["current_phase"]
    current = PHASES[current_idx]
    thresholds = {
        "phase1_foundation": 3.0,
        "phase2_library_engine": 7.0,
        "phase3_streaming": 7.5,
        "phase4_swiftflow": 8.0,
        "phase5_polish": 8.5,
    }
    threshold = thresholds.get(current, 7.0)
    if score >= threshold and tests_ok:
        log(f"{current} meets threshold ({score:.1f} >= {threshold})")
        advance_phase(state)
    else:
        log(f"{current} below threshold ({score:.1f} < {threshold}), continuing work")
    
    # Build and send Telegram report
    current_idx = state["current_phase"]
    current = PHASES[current_idx]
    report = f"""*AetherStream Report*

Phase: `{current}`
Status: {state["phase_status"][current]}
Health: {state["health_score"]:.1f}/10
Build: {"OK" if state["build_ok"] else "FAIL"}
Tests: {"OK" if state["tests_ok"] else "FAIL"}
Crashes: {len(state["crashes"])}

Next run: +1h"""
    send_telegram(report)

    save_state(state)
    log("Orchestrator complete")

if __name__ == "__main__":
    main()
