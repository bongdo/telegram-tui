# TODO

Legend: **P0** = next, blocks daily use; **P1** = important; **P2** = later.
Each item includes why, approach and acceptance.

---

## P0 — responsiveness

### PERF-1 (done): wheel scroll coalescing and message canvas cache

Wheel deltas accumulate per panel and apply once on a 16 ms tick. The message
canvas is cached by peer, width, theme, message version, selection and voice
frame; scrolling only slices cached lines. Hover/selection invalidate only when
the hovered message actually changes. Benchmarks live in
`internal/ui/model_test.go` (`BenchmarkMessageView*`, `BenchmarkView*`): cached
canvas reads are ~0.1 µs, a full canvas rebuild ~0.7 ms, a full `View` ~0.42 ms
on the 120×40 mock conversation.

**Remaining:** scripted pty backlog test.

### PERF-2 (done): mouse sweep over the chat list lags behind the pointer

Two causes, both fixed:

- `paint`/`fill` built a lipgloss style per cell, and `Style.Render`'s property
  lookups dominated frame cost. They now emit the same SGR through
  `ansi.Style` directly: a full `View` dropped from ~1.15 ms to ~0.42 ms.
- The terminal (all-motion mode) emits many motion events for the cell the
  pointer already sits on. `ui.Filter` drops those before `Update`, so they
  cost neither a frame nor an event-loop slot; movement to a new cell is always
  delivered, so hover never sticks.

**Remaining:** `scripts/pty-smoke.sh lag` (tmux pty, demo mode) now measures the
real thing: a 30-move sweep reacts in ~30-85 ms and settles in the same frame.
That harness has a ~25-55 ms floor of its own (one tmux spawn per poll), so it
confirms "no backlog" but cannot resolve the 2.6× CPU win — `BenchmarkView*`
does that.

### PERF-3: frame budget and backpressure

**Approach**
- Add lightweight timing around `View` and event handling (debug-only).
- Cap render rate with `tea.WithFPS` if needed.
- Ensure network/media work never happens in `Update`/`View`; all heavy work is
  a `tea.Cmd`/goroutine.
- Document a “no blocking in View” rule.

**Acceptance:** a long conversation can be scrolled while a media download and a
Telegram update arrive without a visible stall.

---

## P0 — real folders and archive

### FOLDERS-1 (done): account folders and archive

`messages.getDialogFilters` is mapped to domain folders (title, emoji, pinned,
include/exclude peers, category flags), the main list and `folder_id=1` archive
are loaded separately, and membership is computed from the filter rules.
Folder definitions are persisted in SQLite; tabs show All, Archive and custom
folders with unread counts. Mock tabs remain only in demo mode. The tab row
shows a window around the selected folder; wheel over the row cycles folders.

**Remaining:** live `updateDialogFilter*` handling and per-folder pagination
beyond the first 100 dialogs.

### FOLDERS-2: server-side mute/pin and dialog state

**Why:** current context actions only mutate the local model.

**Approach:** `messages.toggleDialogPin`, `account.updateNotifySettings` (or
`messages.updateDialogFilter` where relevant); reconcile via updates. Show
pending state and roll back on error.

**Acceptance:** pin/mute changes are reflected on other devices and survive
restart.

---

## P0 — composer text selection

### INPUT-1 (done): double-click word selection in the composer

**Why:** double-click already replies to a message; the text box has no mouse
selection. Users expect to select/copy text.

**Approach**
- Bubbles v2 textarea exposes `BeginSelection`, `ExtendSelection`, `EndSelection`,
  `ClearSelection`, `HasSelection`.
- Map mouse cells to visual text positions, accounting for wrapping and the
  composer’s inner padding.
- Double-click selects the word under the pointer; drag extends; triple-click
  selects the line. Copy via `CopySelection`/OSC52.
- Keep double-click on a **message bubble** as reply; composer double-click is
  text selection.
- Do not interfere with the single-click focus behaviour.

**Acceptance:** double-click selects a word in the composer; drag extends the
selection; Ctrl/Cmd+C copies; clicking elsewhere clears it; reply-on-double-click
still works on messages.

**Code:** `internal/ui/composer.go`, `internal/ui/mouse.go` (`centerClick`),
Bubbles v2 `textarea/selection.go` (`BeginSelection`, `ExtendSelection`,
`EndSelection`, `CopySelection`).

---

## P1 — messaging completeness

- **Message selection and multi-select**: visual cursor, shift-click/ranges,
  bulk copy/delete/forward.
- **Edit and forward (done)**: edit own text in the composer with cancel and
  edited state; forward uses a searchable chat picker and keeps attribution.
- **Drafts**: per-chat draft persisted in SQLite and synced where practical.
- **Read receipts and typing**: map `messages.readHistory`, `messages.setTyping`
  and incoming typing updates.
- **Search**: global (`messages.searchGlobal`) and in-chat (`messages.search`),
  jump-to-message, highlight hits.
- **Pinned messages** and jump-to-pinned.
- **Message links**: open `t.me` links, copy links, handle `tg://` URLs.
- **Reconnect UX**: visible reconnecting state, retry budget, no silent loss of
  updates.

## P1 — media and stickers

- **Inline photo thumbnails** in bubbles: async download/thumbnail cache, render
  through chafa/Kitty/Sixel/half-blocks; click opens the full viewer.
- **Sticker packs**: `messages.getStickerSet` / installed packs; static WebP,
  animated TGS (Lottie) and WebM fallback; recent stickers persisted.
- **Custom emoji** in message text and reactions.
- **Round-video frame**: `ffmpeg` frame extraction + chafa; keep audio playback.
- **Upload progress**: percentage and cancel; send as photo/document with
  caption and reply; drag/paste path support.
- **Download manager**: queue, cancel, retry, cache eviction policy.
- **Image protocols**: detect Kitty/Sixel/iTerm2; fall back to half blocks/chafa.

## P1 — identity and sessions

- **Account identity screen**: show the logged-in user, phone/username, session
  location; explicit sign out and “use another account”.
- **Multiple accounts**: separate session files and cache namespaces; switch
  without restarting.
- **Session security**: 0600 files, optional app passcode, clear confirmation
  before sign-out.

## P1 — profiles and groups

- Full user/group/channel info (`users.getFullUser`, `channels.getFullChannel`).
- Members list, admin roles, shared media, pinned messages.
- Group/channel management actions behind a confirmation dialog.

## P2 — platform

- Bots: commands, inline keyboards, callback queries; inline mode.
- Polls, contacts, locations, web pages previews.
- Proxies (MTProxy/SOCKS5), debug logging. (Test DC selection is wired via
  `TG_TEST_DC` / `TG_DC`.)
- Config file, themes, keymap customization, notifications.
- Packaging: static binaries, Nix flake/dev shell, release checksums.

---

## Known limitations (current)

- Mute/pin context actions are local only; folder changes are not applied live.
- Inline photos and sticker packs are placeholders; stickers tab is a stub.
- Round notes play audio but do not show a frame.
- Multi-select, drafts and typing are not implemented (edit and forward are).
- Read receipts are always `✓✓`; typing is not shown.
- Dialogs capped at 100 per folder; history at 60 messages.
- Only one account/session per process.
- Recent emoji are client-side only (no such RPC in the current layer).
- Message canvas scroll is cached and coalesced; a full `View` costs ~0.42 ms
  and repeated motion events on the same cell are dropped before `Update`
  (PERF-2).

## Verification plan

- Keep `go test ./...`, `go test -race`, `go vet` green.
- Benchmarks: `BenchmarkView`, `BenchmarkViewHoverSweep`,
  `BenchmarkMessageView*` (canvas rebuild vs cache).
- `scripts/pty-smoke.sh screen` dumps the real rendered screen; `lag` measures
  hover latency in demo mode. Use it before/after render changes and for
  screenshots without a human in the loop.
- Still missing: golden-screen asserts and a wheel-burst variant in the pty
  harness.
- Keep live probes for dialogs/history as a manual smoke test.
- Test with chafa installed and absent; with mpv installed and absent.
- Test auth paths: phone/code/2FA, QR, remembered session, corrupted session,
  sign-out.
- Test offline start from SQLite cache.
