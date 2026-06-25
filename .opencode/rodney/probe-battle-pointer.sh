#!/usr/bin/env bash
# Rodney computed-style guard for AC5: battle-pointer visibility.
# Asserts non-zero dimensions + visible state in REVEALED state.
set -euo pipefail

PORT="${PORT:-8081}"
APP="./battle-bracket-wheels"
RODNEY_LOG="docs/evidence/33/rodney.log"

# Cleanup
cleanup() {
  echo "" | tee -a "$RODNEY_LOG"
  echo "=== Cleanup ===" | tee -a "$RODNEY_LOG"
  uvx rodney stop 2>/dev/null || true
  kill $APP_PID 2>/dev/null || true
  wait $APP_PID 2>/dev/null || true
}
trap cleanup EXIT

# Start fresh log
mkdir -p docs/evidence/33
: > "$RODNEY_LOG"

echo "=== Building app ===" | tee -a "$RODNEY_LOG"
go build -o "$APP" . 2>&1 | tee -a "$RODNEY_LOG"

echo "" | tee -a "$RODNEY_LOG"
echo "=== Starting app on port $PORT ===" | tee -a "$RODNEY_LOG"
PORT="$PORT" "$APP" &
APP_PID=$!
sleep 2

# Verify app is running
echo "" | tee -a "$RODNEY_LOG"
echo "=== Health check ===" | tee -a "$RODNEY_LOG"
curl -sf "http://localhost:${PORT}/health" 2>&1 | tee -a "$RODNEY_LOG" || { echo "App failed to start" | tee -a "$RODNEY_LOG"; exit 1; }

echo "" | tee -a "$RODNEY_LOG"
echo "=== Starting rodney browser ===" | tee -a "$RODNEY_LOG"
uvx rodney start --local 2>&1 | tee -a "$RODNEY_LOG"

echo "" | tee -a "$RODNEY_LOG"
echo "=== Opening app ===" | tee -a "$RODNEY_LOG"
uvx rodney open "http://localhost:${PORT}/" 2>&1 | tee -a "$RODNEY_LOG"

# Wait for page load
uvx rodney waitload 2>&1 | tee -a "$RODNEY_LOG"

echo "" | tee -a "$RODNEY_LOG"
echo "=== Adding option to wheel 0 ===" | tee -a "$RODNEY_LOG"
uvx rodney input "#wheel-slot-1-0 input[name=text]" "Starfighter" 2>&1 | tee -a "$RODNEY_LOG"
uvx rodney click "#wheel-slot-1-0 button[type=submit]" 2>&1 | tee -a "$RODNEY_LOG"
sleep 2

echo "" | tee -a "$RODNEY_LOG"
echo "=== Adding option to wheel 1 ===" | tee -a "$RODNEY_LOG"
uvx rodney input "#wheel-slot-2-1 input[name=text]" "Space Pirate" 2>&1 | tee -a "$RODNEY_LOG"
uvx rodney click "#wheel-slot-2-1 button[type=submit]" 2>&1 | tee -a "$RODNEY_LOG"
sleep 2

echo "" | tee -a "$RODNEY_LOG"
echo "=== Clicking Battle QF1 ===" | tee -a "$RODNEY_LOG"
uvx rodney click "#battle-btn-qf1 button" 2>&1 | tee -a "$RODNEY_LOG"

# Wait for reveal (animation 3.5s + reveal at 3.7s + transition 0.5s + buffer)
echo "" | tee -a "$RODNEY_LOG"
echo "=== Waiting for reveal animation (8s) ===" | tee -a "$RODNEY_LOG"
sleep 8

echo "" | tee -a "$RODNEY_LOG"
echo "=== Checking .battle-pointer computed styles (revealed state) ===" | tee -a "$RODNEY_LOG"
echo "" | tee -a "$RODNEY_LOG"

# Capture computed values
echo "--- Computed values ---" | tee -a "$RODNEY_LOG"
WIDTH=$(uvx rodney js "getComputedStyle(document.querySelector('.battle-pointer')).width" 2>&1)
echo "width: $WIDTH" | tee -a "$RODNEY_LOG"
HEIGHT=$(uvx rodney js "getComputedStyle(document.querySelector('.battle-pointer')).height" 2>&1)
echo "height: $HEIGHT" | tee -a "$RODNEY_LOG"
VISIBILITY=$(uvx rodney js "getComputedStyle(document.querySelector('.battle-pointer')).visibility" 2>&1)
echo "visibility: $VISIBILITY" | tee -a "$RODNEY_LOG"
OPACITY=$(uvx rodney js "getComputedStyle(document.querySelector('.battle-pointer')).opacity" 2>&1)
echo "opacity: $OPACITY" | tee -a "$RODNEY_LOG"
DISPLAY=$(uvx rodney js "getComputedStyle(document.querySelector('.battle-pointer')).display" 2>&1)
echo "display: $DISPLAY" | tee -a "$RODNEY_LOG"

echo "" | tee -a "$RODNEY_LOG"
echo "--- Assertions ---" | tee -a "$RODNEY_LOG"

ASSERT_FAIL=0

# Width > 0
if uvx rodney assert "parseFloat(getComputedStyle(document.querySelector('.battle-pointer')).width) > 0" 2>&1 | tee -a "$RODNEY_LOG"; then
  echo "  ✓ pointer width > 0" | tee -a "$RODNEY_LOG"
else
  echo "  ✗ pointer width > 0: FAILED" | tee -a "$RODNEY_LOG"
  ASSERT_FAIL=1
fi

# Height > 0
if uvx rodney assert "parseFloat(getComputedStyle(document.querySelector('.battle-pointer')).height) > 0" 2>&1 | tee -a "$RODNEY_LOG"; then
  echo "  ✓ pointer height > 0" | tee -a "$RODNEY_LOG"
else
  echo "  ✗ pointer height > 0: FAILED" | tee -a "$RODNEY_LOG"
  ASSERT_FAIL=1
fi

# Visibility === 'visible'
if uvx rodney assert "getComputedStyle(document.querySelector('.battle-pointer')).visibility === 'visible'" 2>&1 | tee -a "$RODNEY_LOG"; then
  echo "  ✓ pointer visibility visible" | tee -a "$RODNEY_LOG"
else
  echo "  ✗ pointer visibility visible: FAILED" | tee -a "$RODNEY_LOG"
  ASSERT_FAIL=1
fi

# Opacity !== '0'
if uvx rodney assert "parseFloat(getComputedStyle(document.querySelector('.battle-pointer')).opacity) !== 0" 2>&1 | tee -a "$RODNEY_LOG"; then
  echo "  ✓ pointer opacity non-zero" | tee -a "$RODNEY_LOG"
else
  echo "  ✗ pointer opacity non-zero: FAILED" | tee -a "$RODNEY_LOG"
  ASSERT_FAIL=1
fi

# Display !== 'none'
if uvx rodney assert "getComputedStyle(document.querySelector('.battle-pointer')).display !== 'none'" 2>&1 | tee -a "$RODNEY_LOG"; then
  echo "  ✓ pointer display not none" | tee -a "$RODNEY_LOG"
else
  echo "  ✗ pointer display not none: FAILED" | tee -a "$RODNEY_LOG"
  ASSERT_FAIL=1
fi

echo "" | tee -a "$RODNEY_LOG"
if [ "$ASSERT_FAIL" -eq 0 ]; then
  echo "=== ALL ASSERTIONS PASSED ===" | tee -a "$RODNEY_LOG"
else
  echo "=== SOME ASSERTIONS FAILED ($ASSERT_FAIL) ===" | tee -a "$RODNEY_LOG"
  exit 1
fi
