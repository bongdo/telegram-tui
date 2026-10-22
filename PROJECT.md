# tg-matrix-tui — project brief

A mouse-first Telegram **user** client that runs in the terminal. It is not a
bot UI and not a Matrix bridge: the directory name is historical, the product is
a Telegram client.

The visual and behavioural reference is Telegram Web A (`web.telegram.org/a`),
with Telegram Web K and Telegram Desktop used for details the web client hides.
The terminal constraint is treated as a design material, not as a reason to
throw the web model away.

## 1. Principles

1. **Mouse first, keyboard always available.**
   Every primary action must be possible with one hand on a mouse: open a chat,
   send, attach, play, seek, react, reply, delete, switch folder, open profile,
   close overlays, quit. Keybindings are the backup path, never the only path.
2. **Web fidelity where it matters.**
   Three-column shell, a 420-ish px chat list, 728-ish px conversation, profile
   on demand, 56 px header, rounded bubbles, own-bubble colour, folder pills,
   composer layout. The TUI maps these to cell geometry rather than imitating
   pixels.
3. **Honest UI.**
   If a feature is not wired, it says so. No fake success, no buttons that
   silently do nothing, no mock data in real mode.
4. **Local-first rendering.**
   The UI never blocks on the network. Telegram events arrive through a bridge;
   cached chats and messages render before the network answers.
5. **Rebuildable cache, precious session.**
   Message cache can be deleted at any time. The gotd session is the only
   credential and is treated as a secret.
6. **Procedural, explicit code.**
   Data structs in `internal/domain`; behaviour in plain functions; the UI and
   the bridge do not import each other's internals.

## 2. Current state

Implemented and smoke-tested against a real account:

- **TUI shell**: Bubble Tea v2, Bubbles v2 (`textinput`, `textarea`), Lip Gloss
  v2. Wide 3-column layout and compact 1-pane layout (`<84` columns), profile
  panel (`>=100` columns), status bar.
- **Chat list**: avatars, previews, time, unread pills, pinned/muted/verified
  markers, hover, wheel scroll, search filtering, chat context menu.
- **Folders and archive**: real `messages.getDialogFilters`, All + Archive +
  custom folder tabs with unread counts, client-side membership from filter
  rules, archive loaded through `folder_id=1`, definitions persisted in SQLite.
- **Conversation**: date separators, incoming/outgoing bubbles, reply and
  forward previews, sender names in groups, reactions, views, edited marker,
  checks, wheel scroll, keyboard scroll. The message canvas is cached by peer,
  width, theme, message version, selection and voice frame; rapid wheel deltas
  are coalesced into one 16 ms tick. A full `View` on the 120×40 mock costs
  ~0.42 ms, and duplicate mouse-motion events for the current cell are dropped
  in `ui.Filter` before `Update`.
- **Composer**: multiline input, Enter send, Shift+Enter newline, reply bar,
  attach, emoji, send. Click-to-focus, hotkeys.
- **Voice/round/photo**: voice waveform + play/pause/seek through `mpv` JSON
  IPC; round-note audio; photo viewer through `chafa` (`tea.Cmd`, never inside
  `View`); media downloaded on demand.
- **Emoji**: 8×4 panel, stickers tab placeholder, client-side recently used
  emoji persisted in SQLite.
- **Context menus**: right-click or `m`; quick reactions, reply, copy, edit,
  forward picker, delete; chat open/read/mute/pin (mute/pin are local for now).
- **Auth**: phone/code/2FA and QR login, chosen in a modal. QR rendered in the
  terminal via `qrterminal`; 2FA fallback after QR. Sign-up intentionally
  rejected.
- **Telegram bridge**: gotd/td user client, file session storage, raw
  `messages.getDialogs` for the main list and archive, `getDialogFilters`,
  `getHistory`, live updates (new/edit/delete/reactions), text send, replies,
  edits, forwards, uploads, reactions, delete, read history, media download.
  Flood-wait middleware plus explicit retries; status bar shows the countdown.
  `TG_DC` picks the primary DC and `TG_TEST_DC=1` switches to Telegram's test
  DCs with isolated `*-test` session, cache and media paths.
- **Persistence**: `session.json` for the gotd key; SQLite `cache.db` (WAL) for
  chats, recent messages, media paths and recent emoji; media files under the
  user cache directory.
- **Demo mode**: no credentials means mock chats and demo audio/image paths. In
  real mode the mock list is not shown.

Verified live: 31 dialogs and 61 messages loaded from the account; QR panel
rendered (not scanned). Tests cover layout widths, hit-testing, overlays, auth
modal, QR panel, event application, conversion, store round-trip, chafa; race
tests for UI/bridge/store. Manual checks: mpv playback and seek. The real binary
can be driven in a tmux pty by `scripts/pty-smoke.sh`: `screen` dumps the
rendered frame, `lag` measures hover latency after a motion burst (demo mode).

## 3. Architecture

```
┌────────────────────────────── internal/ui ──────────────────────────────┐
│ model (Bubble Tea)                                                      │
│   geometry: layout.go      mouse: mouse.go                              │
│   panels:   render.go, conversation.go, composer.go, profile.go         │
│   overlays: overlay.go (ctx menu, emoji), attach_ui.go, tg_ui.go (auth) │
│   mock:     data.go                                                     │
└───────────────▲───────────────────────────────┬─────────────────────────┘
                │ domain.Chat / domain.Message  │ Bridge commands
                │ tea.Msg events                │
┌───────────────┴───────────────────────────────▼─────────────────────────┐
│ internal/tgclient: gotd/td bridge                                       │
│   auth (phone/QR/2FA), dialogs, history, updates, send/upload/react/    │
│   delete, media download, flood-wait handling, typed events             │
└───────────────▲───────────────────────────────┬─────────────────────────┘
                │                               │
        internal/domain                 internal/store (SQLite)
        shared data types               chats, messages, media, recent emoji
```

Supporting packages:

- `internal/audio`: persistent `mpv` process + JSON IPC. Progress is tracked in
  Go against the message duration; pause/seek are forwarded to mpv.
- `internal/chafa`: image → ANSI through the `chafa` CLI for the media viewer.
- `internal/store`: pure-Go SQLite (`modernc.org/sqlite`), WAL, JSON payload per
  row for schema flexibility.

Event flow: gotd goroutine → `tgclient` channel → `waitBridge` tea.Cmd → model
Update. UI never calls MTProto directly. Commands (send, download, upload) are
fire-and-forget goroutines that emit typed events back.

## 4. Persistence

| Data | Location | Format | Notes |
|---|---|---|---|
| gotd session | `~/.config/tg-matrix-tui/session.json` | JSON | secret, 0600 |
| chats, messages, media paths, recent emoji | `~/.config/tg-matrix-tui/cache.db` | SQLite WAL | rebuildable |
| media | `~/.cache/tg-matrix-tui/media/` | original files | redownloadable |
| UI state | memory | — | not persisted yet |

Overrides: `TG_SESSION`, `TG_DB`, `TG_CACHE`. `TG_TEST_DC=1` adds a `-test`
suffix to the default paths so test and production credentials never mix. The
session is intentionally not in SQLite: it is gotd's serialized auth key and
should be easy to delete or protect separately.

SQLite tables: `chats`, `messages`, `recent_emoji`. Messages are stored as JSON
payloads with `(peer_key, id, date)` for ordering. History refresh replaces the
per-chat message set; new messages upsert. Media paths are written back into the
message JSON.

## 5. Key decisions

| Decision | Why |
|---|---|
| Telegram Web A as primary reference | Most mature desktop web layout; documented tokens; 3-column model maps to cells |
| Web K as secondary | Compact/small-screen behaviour and folder patterns |
| Bubble Tea v2 + Bubbles v2 + Lip Gloss v2 | Elm loop, mature inputs, style/layer compositing, Charm ecosystem |
| gotd/td over TDLib | Pure Go, no cgo, no external C++ runtime, full MTProto |
| Raw RPC for dialogs/history | Streaming iterators plus flood-wait middleware produced long waits; one request + explicit retry is predictable |
| mpv over in-process Opus | Handles Ogg/Opus, MP4 audio, pause/seek today; `tele` shows the pure-Go upgrade path |
| chafa for images | Mature ANSI renderer; external process keeps the TUI binary simple |
| SQLite cache + JSON session | Cache can be invalidated; session stays isolated and easy to protect |
| Central mouse router | One hit-test source of truth; panels stay position-agnostic |
| Mock fallback | Allows UI work without credentials; never shown in real mode |

## 6. References

- [Ajaxy/telegram-tt](https://github.com/Ajaxy/telegram-tt) — Telegram Web A.
- [telegram-tt themes.json](https://github.com/Ajaxy/telegram-tt/blob/master/src/styles/themes.json)
  — exact light/dark colours.
- [morethanwords/tweb](https://github.com/morethanwords/tweb) — Web K.
- [telegramdesktop/tdesktop chat.style](https://github.com/telegramdesktop/tdesktop/blob/dev/Telegram/SourceFiles/ui/chat/chat.style)
  — message metrics.
- [strawberry-code/tuilegram](https://github.com/strawberry-code/tuilegram) —
  mouse routing ADR for a Telegram TUI.
- [charmbracelet/crush](https://github.com/charmbracelet/crush) — mature Bubble
  Tea v2 interaction patterns.
- [sorokin-vladimir/tele](https://github.com/sorokin-vladimir/tele) — pure-Go
  voice notes, inline images, round notes.
- [gotd/td](https://github.com/gotd/td) — MTProto client.
- [chafa](https://hpjansson.org/chafa/) — image → ANSI.
- [qrterminal](https://github.com/mdp/qrterminal) — QR rendering.

Detailed web/token mapping: `docs/telegram-web-reference.md`.

## 7. Known gaps

- Mute/pin are local-only; no server reconciliation or live folder updates.
- Inline photo thumbnails, sticker packs, custom emoji and round-video frames
  are placeholders.
- No multi-select, drafts, typing indicators or real read receipts.
- Chat-list hover costs a full `View` (~0.42 ms); identical motion events are
  dropped, but hover still re-renders the list rather than updating one row.
- One account/session per process; no sign-out/switch-account screen.
- Dialogs capped at 100 per folder, history at 60 messages.
- Recent emoji are client-side only (no corresponding RPC in the current layer).
- Folder tabs show a width-limited window around the selected tab; wheel over
  the tab row cycles the rest.

## 8. Final product

A complete terminal Telegram client that feels like Telegram Web A, not like a
CLI that happens to show messages.

**Daily-use definition of done**

- Real account(s): phone/QR login, remembered session, visible identity, sign
  out / switch account.
- My folders, not mock tabs: All, custom folders, Archive, unread counts,
  ordering, pinned dialogs; server-side mute/pin.
- Fast, responsive list and conversation: wheel and hover never queue frames;
  scroll target jumps immediately; stable 60 fps while moving the mouse.
- Complete message loop: send, reply, edit, delete, forward, copy, multi-select,
  reactions, drafts, read receipts, typing indicators.
- Media: inline photo thumbnails, sticker packs (static + animated), custom
  emoji, voice/round playback, file upload/download with progress.
- Search: global and in-chat, jump to message.
- Profiles and groups: full user/group/channel info, members, shared media,
  pinned messages.
- Reliability: reconnect without losing the session; cache survives restarts;
  flood waits are visible and survive without dropping to demo data.
- Terminal UX: mouse-first with complete keyboard parity, compact mode, themes,
  configurable media backends (chafa/Kitty/Sixel), no blocking on the network.

**Non-goals for the first “complete” release**

Voice/video calls, secret chats, stories, payments, full bot platform, and a
pixel-perfect clone of Telegram Desktop. They can be separate milestones.

**Success signals**

- Rapid scroll/hover feels instant; no “waiting for animations”.
- All primary actions work with mouse only and with keyboard only.
- Restart with no network still shows my chats and recent messages from cache.
- A full day of use is possible without a GUI client for text conversations.
