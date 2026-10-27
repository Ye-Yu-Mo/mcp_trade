#!/bin/bash
# Keeps the Mac→VPS SSH forwards for the trading server alive.
#
# Why this exists: the ssh forward is not a durable link. It dies on Wi-Fi
# changes, on sleep/wake, and — the nastiest case — silently: the ssh process
# stays alive while the forwarded ports stop listening, which looks exactly like
# "the MCP server is broken". launchd KeepAlive only restarts a process that
# actually exited, so a port-level check is the only thing that catches that.
#
# Run by com.jasxu.mcp-trade-tunnel-watch every 2 minutes, and safe to run by
# hand or from cron (`*/2 * * * * /path/to/watch.sh`).

set -u

LABEL="gui/$(id -u)/com.jasxu.mcp-trade-tunnel"
PORTS=(8877 8878 8879)   # 8877 live · 8878 paper · 8879 paper-5m
LOG="/Users/jasxu/Library/Logs/mcp-trade.tunnel.watch.log"

port_dead() {
  ! /usr/bin/nc -z -w 2 127.0.0.1 "$1" >/dev/null 2>&1
}

any_dead() {
  local p
  for p in "${PORTS[@]}"; do
    port_dead "$p" && return 0
  done
  return 1
}

any_dead || exit 0

# One retry: a restarting tunnel can be down for a fraction of a second.
/bin/sleep 5
any_dead || exit 0

echo "$(date -u +%FT%TZ) tunnel down on one of ${PORTS[*]}; kickstarting $LABEL" >>"$LOG"
/bin/launchctl kickstart -k "$LABEL"
