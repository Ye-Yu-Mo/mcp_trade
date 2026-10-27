#!/bin/bash
# Keeps the Mac→VPS SSH forwards for the trading server alive.
#
# Why this exists: the ssh forward is not a durable link. It dies on Wi-Fi
# changes, on sleep/wake, and — the nastiest case — silently: the ssh process
# stays alive and the local ports still accept TCP connections, but no data
# ever comes back, so every MCP call hangs until it times out. launchd
# KeepAlive only restarts a process that actually exited, so the check has to
# be an HTTP request with a deadline, not a port probe. `nc -z` reports "up"
# in exactly the broken state this script exists to fix (observed 2026-10-27).
#
# Run by com.jasxu.mcp-trade-tunnel-watch every 2 minutes, and safe to run by
# hand or from cron (`*/2 * * * * /path/to/watch.sh`).

set -u

LABEL="gui/$(id -u)/com.jasxu.mcp-trade-tunnel"
PORTS=(8877 8878 8879)   # 8877 live · 8878 paper · 8879 paper-5m
LOG="/Users/jasxu/Library/Logs/mcp-trade.tunnel.watch.log"

# /health is public and returns {"status":"ok",...} — any answer means the
# forward is passing bytes, which is what we actually care about.
healthy() {
  /usr/bin/curl -s -m 6 -o /dev/null -w "%{http_code}" "http://127.0.0.1:$1/health" 2>/dev/null | grep -q "^200$"
}

any_broken() {
  local p
  for p in "${PORTS[@]}"; do
    healthy "$p" || return 0
  done
  return 1
}

any_broken || exit 0

# One retry: a freshly restarted tunnel can be a second or two from serving.
/bin/sleep 5
any_broken || exit 0

echo "$(date -u +%FT%TZ) tunnel unhealthy on one of ${PORTS[*]}; kickstarting $LABEL" >>"$LOG"
/bin/launchctl kickstart -k "$LABEL"
