#!/usr/bin/env bash
# Drive the real TUI inside a tmux pty, like a robot user.
#
#   scripts/pty-smoke.sh screen [cols] [rows] [out]   dump the rendered screen
#   scripts/pty-smoke.sh lag    [cols] [rows] [moves] measure hover latency
#
# `lag` moves the pointer over the chat list in one burst and reports, for three
# runs: `first` — ms until the screen reacts at all, `settle` — ms until the
# screen matches the final state (no backlog).
#
# Limits: every poll spawns tmux (~7 ms), so a single motion event already
# measures 25-55 ms here; treat that as the floor, not as app latency. Runs in
# demo mode (TG_TUI_DEMO=1) so the result never depends on an account or cache.
# Needs tmux; if it is missing the script re-executes under nix-shell.
set -euo pipefail

if ! command -v tmux >/dev/null; then
  exec nix-shell -p tmux --run "$(printf '%q ' "$0" "$@")"
fi

cd "$(dirname "$0")/.."
go build -o telegram-tui .

mode=${1:-screen}
cols=${2:-120}
rows=${3:-40}
sock="tg-pty-$$"
trap 'tmux -L "$sock" kill-server 2>/dev/null || true' EXIT

tmux -L "$sock" new-session -d -x "$cols" -y "$rows" env TG_TUI_DEMO=1 ./telegram-tui
capture() { tmux -L "$sock" capture-pane -p -e; }
now_ms() { date +%s%3N; }
send() { tmux -L "$sock" send-keys -H "$@"; }

# Wait for the first frame (status bar); the app starts faster than tmux settles.
for _ in $(seq 1 100); do
  capture | grep -q Telegram && break
  sleep 0.05
done
sleep 0.3

if [ "$mode" = screen ]; then
  capture >"${4:-/dev/stdout}"
  exit 0
fi

moves=${4:-30}
# SGR motion (no button): ESC [ < 35 ; x ; y M, 1-based cells. Demo chats fill
# rows 5..25 (two lines each) and the burst ends on row 13, a chat row that
# always exists, so the final hover is guaranteed to be visible. Bytes are built
# once; building them inside the timed section would measure subprocess cost.
build_burst() {
  local i row seq=""
  for ((i = 0; i < moves; i++)); do
    row=$((5 + (i % 11) * 2))
    [ "$i" -eq $((moves - 1)) ] && row=13
    seq+="\033[<35;$((4 + i % 20));${row}M"
  done
  printf '%b' "$seq" | od -An -tx1 -v | tr -s ' \n' ' '
}
BURST=$(build_burst)
NEUTRAL=$(printf '\033[<35;2;2M' | od -An -tx1 -v | tr -s ' \n' ' ')
# shellcheck disable=SC2086 # word splitting is the point: one hex byte per word
burst() { send $BURST; }

# Reference: what the screen looks like once the burst has fully landed.
burst
sleep 0.5
ref=$(capture)

# Neutral pointer position outside the chat rows, then measure the same burst.
# shellcheck disable=SC2086
send $NEUTRAL
sleep 0.3
base=$(capture)
[ "$base" = "$ref" ] && { echo "hover is not visible: chat list empty or too short" >&2; exit 1; }

echo "hover burst: $moves moves, ${cols}x${rows}, 3 runs"
for run in 1 2 3; do
  # shellcheck disable=SC2086
  send $NEUTRAL
  sleep 0.25
  base=$(capture)
  t0=$(now_ms)
  burst
  first=0
  while :; do
    cur=$(capture)
    t=$(( $(now_ms) - t0 ))
    if [ "$first" = 0 ] && [ "$cur" != "$base" ]; then first=$t; fi
    if [ "$cur" = "$ref" ]; then break; fi
    if [ "$t" -gt 3000 ]; then
      echo "run $run: screen never settled (${t} ms)" >&2
      exit 1
    fi
    sleep 0.001
  done
  echo "run $run: first ${first} ms, settle ${t} ms"
done
