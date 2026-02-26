#!/usr/bin/env bash
# e2e_test.sh — End-to-end test for blueiris_exporter
#
# Builds the binary, starts it pointed at the integration test log directory,
# hits /metrics, and asserts expected metric values.
#
# Usage (from repo root):
#   ./scripts/e2e_test.sh
#
# Override the listen port if 19876 is already in use:
#   PORT=19877 ./scripts/e2e_test.sh
#
# Required: go (build tag LINUX), curl

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TESTDATA="$REPO_ROOT/blueiris/testdata/integration"
PORT="${PORT:-19876}"
METRICS_URL="http://localhost:${PORT}/metrics"

# ── Working directories ───────────────────────────────────────────────────────
# The log directory and the binary must be kept separate: BlueIris() picks the
# newest file in the log directory by mtime, so anything else (e.g. the binary)
# placed there would be read as a log file.
LOG_DIR="$(mktemp -d)"       # contains only the two integration log files
BIN_DIR="$(mktemp -d)"       # contains only the compiled binary
BINARY="$BIN_DIR/blueiris_exporter"
SERVER_PID=""

cleanup() {
    if [[ -n "$SERVER_PID" ]]; then
        kill "$SERVER_PID" 2>/dev/null || true
        wait "$SERVER_PID" 2>/dev/null || true
    fi
    rm -rf "$LOG_DIR" "$BIN_DIR"
}
trap cleanup EXIT

# ── Prepare log files ─────────────────────────────────────────────────────────
cp "$TESTDATA/integration_test_prior.log" "$LOG_DIR/"
cp "$TESTDATA/integration_test.log"       "$LOG_DIR/"

# integration_test_prior.log must be older so the scanner picks integration_test.log
touch -t 202401010000 "$LOG_DIR/integration_test_prior.log"  # 2024-01-01 00:00
touch -t 202401010100 "$LOG_DIR/integration_test.log"        # 2024-01-01 01:00

# ── Build ─────────────────────────────────────────────────────────────────────
echo ">>> Building..."
cd "$REPO_ROOT"
go build -tags LINUX -o "$BINARY" .
echo "    Binary: $BINARY"

# ── Start exporter ────────────────────────────────────────────────────────────
echo ">>> Starting exporter on :${PORT}..."
"$BINARY" --logpath="$LOG_DIR" --telemetry.addr=":${PORT}" &
SERVER_PID=$!

# ── Wait for /metrics to become available (up to 10 s) ───────────────────────
echo ">>> Waiting for /metrics..."
for i in $(seq 1 40); do
    if curl -sf --max-time 1 "$METRICS_URL" > /dev/null 2>&1; then
        echo "    Ready (${i} x 250 ms)"
        break
    fi
    if [[ $i -eq 40 ]]; then
        echo "ERROR: server did not become ready within 10 s" >&2
        exit 1
    fi
    sleep 0.25
done

# ── Fetch metrics ─────────────────────────────────────────────────────────────
METRICS="$(curl -sf --max-time 5 "$METRICS_URL")"

# ── Assertion helpers ─────────────────────────────────────────────────────────
PASS=0
FAIL=0

check() {
    local desc="$1"
    local pattern="$2"
    if echo "$METRICS" | grep -qE "$pattern"; then
        printf "  PASS  %s\n" "$desc"
        PASS=$((PASS + 1))
    else
        printf "  FAIL  %s\n" "$desc"
        printf "        expected: %s\n" "$pattern"
        FAIL=$((FAIL + 1))
    fi
}

echo ""
echo "=== Metric assertions ==="

# ── AI counters ───────────────────────────────────────────────────────────────
check "ai_timeout       = 1"   '^blueiris_ai_timeout 1$'
check "ai_error         = 1"   '^blueiris_ai_error 1$'
check "ai_notresponding = 1"   '^blueiris_ai_notresponding 1$'
check "ai_restarted     = 1"   '^blueiris_ai_restarted 1$'
check "ai_starting      = 2"   '^blueiris_ai_starting 2$'
check "ai_started       = 2"   '^blueiris_ai_started 2$'
check "ai_servererror   = 1"   '^blueiris_ai_servererror 1$'

# ── Log error / warning totals ────────────────────────────────────────────────
check "logerror_total   = 2"   '^blueiris_logerror_total 2$'
check "logwarning_total = 2"   '^blueiris_logwarning_total 2$'

# ── Triggers ──────────────────────────────────────────────────────────────────
# FD: MOTION_A + EXTERNAL + DIO + Trigger: Motion_A = 4
check 'triggers FD              = 4'  'blueiris_triggers\{camera="FD"\} 4$'
check 'triggers GarageExterior  = 1'  'blueiris_triggers\{camera="GarageExterior"\} 1$'
check 'triggers SouthEast       = 1'  'blueiris_triggers\{camera="SouthEast"\} 1$'

# ── Push notifications ────────────────────────────────────────────────────────
# Labels in Desc order: camera, status, detail
# Prometheus sorts label names alphabetically: camera, detail, status
check 'push FD  OK    = 1'  "blueiris_push_notifications\{camera=\"FD\",detail=\"Garret's S21 Ultra\",status=\"OK\"\} 1\$"
check 'push DW  Error = 1'  "blueiris_push_notifications\{camera=\"DW\",detail=\"Garret's S21 Ultra\",status=\"Error\"\} 1\$"

# ── Profile ───────────────────────────────────────────────────────────────────
# Log switches Day → Night; Day resets to 0, Night becomes 1
check 'profile Night = 1'   'blueiris_profile\{profile="Night"\} 1$'
check 'profile Day   = 0'   'blueiris_profile\{profile="Day"\} 0$'

# ── Camera status ─────────────────────────────────────────────────────────────
# Labels in Desc order: camera, detail
# DW: restored(level-4)→0, network-retry(level-1)→1, Socket-closed(level-1)→1  final=1
check 'camera_status DW  = 1 (Signal: Socket closed)'  'blueiris_camera_status\{camera="DW",detail="Socket closed"\} 1$'
# BY: level-4 link-down → HasPrefix("4") branch → 0
check 'camera_status BY  = 0 (Signal: link down)'      'blueiris_camera_status\{camera="BY",detail="link down"\} 0$'
# FD: Contains("restored") branch → 0
check 'camera_status FD  = 0 (Signal: restored)'       'blueiris_camera_status\{camera="FD",detail="restored"\} 0$'

# ── AI duration (presence only — exact duration depends on last match) ────────
# Prometheus sorts label names alphabetically: camera, detail, object, type
check 'ai_duration FD         alert present'    'blueiris_ai_duration\{camera="FD",[^}]*type="alert"'
check 'ai_duration GATE-MASK  canceled present' 'blueiris_ai_duration\{camera="GATE-MASK",[^}]*type="canceled"'
check 'ai_duration EastNorth  canceled present' 'blueiris_ai_duration\{camera="EastNorth",[^}]*type="canceled"'
check 'ai_duration DW         canceled present' 'blueiris_ai_duration\{camera="DW",[^}]*type="canceled"'

# ── Disk stats ────────────────────────────────────────────────────────────────
# Presence check — exact byte values depend on unit conversion
check 'folder_disk_free  Data Drive'  'blueiris_folder_disk_free\{folder="Data Drive"\}'
check 'folder_disk_free  Alerts'      'blueiris_folder_disk_free\{folder="Alerts"\}'
check 'folder_disk_free  Stored'      'blueiris_folder_disk_free\{folder="Stored"\}'
check 'folder_disk_free  Aux 7'       'blueiris_folder_disk_free\{folder="Aux 7"\}'
check 'folder_disk_free  New'         'blueiris_folder_disk_free\{folder="New"\}'
check 'folder_disk_free  Clips'       'blueiris_folder_disk_free\{folder="Clips"\}'
check 'folder_disk_free  Stored2'     'blueiris_folder_disk_free\{folder="Stored2"\}'
check 'hours_used        Alerts'      'blueiris_hours_used\{folder="Alerts"\}'
check 'hours_used        Stored'      'blueiris_hours_used\{folder="Stored"\}'

# ── logerror / logwarning individual entries ──────────────────────────────────
check 'logerror  Your license'   'blueiris_logerror\{error="Your license'
check 'logerror  Connection'     'blueiris_logerror\{error="Connection refused"\}'
check 'logwarning Maintenance'   'blueiris_logwarning\{warning="\[Maintenance'
check 'logwarning HW decode'     'blueiris_logwarning\{warning="HW decode could not start'

# ── Result ────────────────────────────────────────────────────────────────────
echo ""
if [[ $FAIL -gt 0 ]]; then
    echo "RESULT: FAILED — $FAIL check(s) failed, $PASS passed"
    exit 1
fi
echo "RESULT: PASSED — all $PASS checks passed"
