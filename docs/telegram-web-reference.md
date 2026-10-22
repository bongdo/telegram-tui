# Telegram web → TUI reference map

The UI is a port of the **Telegram Web A** layout (`web.telegram.org/a`) with
behaviour borrowed from **Telegram Web K** where it is stricter about small
screens. Telegram Desktop's style files are used for message metrics that the
web clients express only indirectly.

## 1. Sources

| Source | What we use it for |
|---|---|
| [Ajaxy/telegram-tt](https://github.com/Ajaxy/telegram-tt) — Telegram Web A | 3-column shell, CSS tokens, composer, profile panel, chat list rows |
| [Ajaxy/telegram-tt `themes.json`](https://github.com/Ajaxy/telegram-tt/blob/master/src/styles/themes.json) | exact light/dark colour pairs |
| [morethanwords/tweb](https://github.com/morethanwords/tweb) — Telegram Web K | compact behaviour, chat list/folder patterns, bubble grouping |
| [morethanwords/tweb-design](https://github.com/morethanwords/tweb-design) | mockups of the web composition |
| [telegramdesktop/tdesktop `ui/chat/chat.style`](https://github.com/telegramdesktop/tdesktop/blob/dev/Telegram/SourceFiles/ui/chat/chat.style) | message max width, paddings, reply preview, service message spacing |
| [strawberry-code/tuilegram](https://github.com/strawberry-code/tuilegram) | mouse routing design for a Telegram TUI: central hit-test, bbox cache, wheel-by-cursor, keyboard parity |
| [charmbracelet/crush](https://github.com/charmbracelet/crush) | mature Bubble Tea v2 mouse/overlay patterns |
| [sorokin-vladimir/tele](https://github.com/sorokin-vladimir/tele) | pure-Go voice-note playback, Ogg/Opus, round-note handling |
| [gotd/td](https://github.com/gotd/td) | MTProto backend for the real client |

Existing TUI clients worth reading before writing the Telegram layer:
`tuilegram` (gotd + Bubble Tea), `tele` (gotd, audio, inline images),
`telecharm` (gotd + reactive store), `tegal1337/telegram-cli` (TDLib + mpv).

## 2. Web layout metrics

Telegram Web A (desktop):

| Token | Value |
|---|---|
| `--messages-container-width` | `45.5rem` = 728px |
| `--right-column-width` | `26.5rem` = 424px (25vw at 1276–1920px) |
| `--header-height` | `3.5rem` = 56px |
| left column (chat list) | 420px |
| `--border-radius-messages` | `0.9375rem` = 15px |
| chat row | ~72px, 54px avatar |
| composer | rounded box, attach left, emoji/send right |
| bubble max width | ~480px on desktop (85% of container) |

Telegram Desktop uses `msgMaxWidth: 430px`, `msgMinWidth: 160px`,
`msgPadding: 11/8/11/8`, `msgMargin: 16/6/56/2`.

### TUI mapping

| Web | TUI |
|---|---|
| 420px sidebar | 26–30 cells |
| 728px conversation | remaining cells |
| 424px profile panel | 30–32 cells, opened on demand |
| 56px header | 2 terminal rows |
| 72px chat row | 2 rows: name/time, preview/unread |
| avatar 54px | 3×2 coloured cell block with initials |
| rounded 15px bubble | `╭─╮` / `╰─╯` with bubble-coloured edges |
| own bubble `#EEFFDE` / `#766AC8` | same colours as terminal background |
| composer box | 1–4 input rows + rounded border, buttons on the bottom row |
| profile column | header, 9×3 avatar, actions, tabs, info sections |

Breakpoints:
- `≥ 100` columns: 3 columns when the profile is open.
- `< 84` columns: one pane at a time (chat list or conversation), `Esc`/`‹` goes back.

## 3. Colour tokens

Taken from `telegram-tt/src/styles/themes.json` (light, dark):

```
primary            #3390EC  #8774E1
background         #FFFFFF  #212121
background-second. #F4F4F5  #0F0F0F
selected           #F4F4F5  #2C2C2C
text               #000000  #FFFFFF
text-secondary     #707579  #AAAAAA
borders            #DADCE0  #303030
chat-active        #3390EC  #766AC8
own bubble         #EEFFDE  #766AC8
own meta           #4FAE4E  #FFFFFF88
online             #4FAE4E  #4DCD5E
```

The TUI keeps the same pairs and switches with `Ctrl+T` or the ☾/☀ button.

## 4. Behaviour inventory

Web A / Web K behaviour that is already represented or planned:

| Area | Web behaviour | TUI |
|---|---|---|
| chat list | single click opens, hover highlight, unread pill, pinned, muted, folders | same |
| chat list | wheel scrolls under cursor | same |
| search | `/` or Ctrl+F, type to filter | click field or `/`, live filter, `Esc` clears |
| conversation | wheel scrolls messages | same, 3 lines per notch |
| message | click selects, double click replies | same |
| message | right-click context menu | quick reactions, Reply, Copy text, Forward, Delete |
| reactions | pick/toggle a reaction | click an emoji in the menu's reaction row; pills re-render |
| voice | play/pause, waveform playhead, seek | play/pause on ▶, click waveform to seek (mpv) |
| round note | audio + round video | audio via mpv, frame preview next |
| photo | click opens media viewer | full-screen chafa overlay, `Esc`/✕ closes |
| login | phone code or QR | method modal; QR rendered in terminal |
| composer | attach a file | path prompt (`a` / +), uploads photo or document |
| emoji | recently used | local SQLite list, shown first in the panel |
| composer | Enter sends, Shift+Enter newline | same |
| composer | attach / emoji / send buttons | emoji and send work; attach is a stub |
| profile | click title/avatar/ⓘ opens, Esc/✕ closes | same |
| profile | Message/Mute/More actions, Info/Media/Files tabs | same, Media/Files are stubs |
| compact | one pane, back navigation | same |
| search | clear/back control | `✕` inside the field |
| app | quit from the UI | `quit` in the status bar |
| keyboard | every mouse action has a key | `Tab`, `/`, `i`, `e`, `m`, `o`, `1–6`, arrows/j/k, `r`, `Esc`, `q` |

Deliberate v0 omissions: forwarding, file upload, calls, sticker packs, custom
emoji, inline message photos/video, search-in-chat. Placeholders are shown
instead of pretending the action happened.

## 5. Mouse architecture

- One central hit-test in `internal/ui/mouse.go`; panels do not know their
  absolute coordinates.
- Geometry is derived from the same `layout()` used by `View()`.
- Wheel target is chosen by cursor position, not focus.
- Clicks change focus and perform the action in one step.
- Overlay/profile gets first priority; clicking outside a dismissable panel
  closes it.
- Motion is enabled (`MouseModeAllMotion`) for hover; all motion-dependent
  visuals degrade gracefully without a mouse.
- Keyboard parity is an invariant: a mouse-only feature is not considered done.

This mirrors tuilegram's ADR-020 and Crush's component hit-testing.

## 6. Audio

Current implementation: `internal/audio` owns one persistent `mpv --idle`
process and talks JSON IPC (`loadfile`, `pause`, `seek`, `stop`, `quit`).
Progress is tracked in Go against the message duration, so the UI does not need
mpv property events. This handles Telegram voice notes (Ogg/Opus), audio files
and the audio track of round video notes without cgo.

Try it with any local file:

```sh
TG_TUI_DEMO_AUDIO=/path/to/voice.ogg go run .
```

then click a voice message. The mock messages have no files, so without the env
variable the status bar explains what is missing.

Pure-Go alternative (no external player): `tele` decodes Ogg/Opus with
`pion/opus` and plays through PulseAudio/PipeWire (`jfreymuth/pulse`) on Linux
and `ebitengine/oto` elsewhere. That is the upgrade path if a static binary
with no `mpv` dependency becomes a requirement. The player interface is
isolated so the backend can be swapped.

Round video notes (`кружочки`) cannot be rendered as video in a terminal v0.
They display as a round-note message with duration; their audio track uses the
same mpv path:

```sh
TG_TUI_DEMO_ROUND_VIDEO=/path/to/round.mp4 go run .
```

## 6.1 Media viewer (chafa)

Photos open in a full-screen overlay rendered by
[chafa](https://hpjansson.org/chafa/): `--format=symbols --colors=256 --size=WxH
--stretch --polite=on`. The command runs as a `tea.Cmd`, never inside `View`, so
the first paint shows a loading row and the render does not block input.

```sh
TG_TUI_DEMO_IMAGE=/path/to/photo.png go run .
```

Without chafa or an image path the viewer explains what is missing. Inline photo
bubbles stay textual until the real Telegram media cache exists; the viewer is
the integration point for Kitty/Sixel later.

## 7. Telegram backend

Implemented with [gotd/td](https://github.com/gotd/td) (pure Go, no TDLib, no
cgo). The bridge lives in `internal/tgclient` and owns every MTProto type; the
UI only sees `internal/domain` events.

- **Auth**: the gotd flow is fed by the TUI through a channel. First the UI asks
  phone or QR. Phone uses `Phone`/`Code`/`Password` prompts; QR uses
  `client.QR().Auth` and renders the token with `qrterminal`, falling back to a
  2FA password prompt. Sign-up is rejected on purpose.
- **Session**: `session.FileStorage` at
  `~/.config/tg-matrix-tui/session.json` by default.
- **DC**: `TG_DC` selects the primary DC (1–5, default 2). `TG_TEST_DC=1`
  uses gotd's test DC list and the `-test` session/cache/media defaults.
- **Cache**: a local SQLite database (`TG_DB`, default
  `~/.config/tg-matrix-tui/cache.db`, WAL) stores chats, recent messages, media
  paths and recently used emoji. It is rebuildable; the session file is the only
  credential. Media files live in `TG_CACHE` (default under
  `~/.cache/tg-matrix-tui/media`).
- **Dialogs**: `query.GetDialogs`, capped at 100 by the initial request, archived chats skipped.
  Users/groups go to the Chats folder, channels to Channels. Pinned, unread and
  muted flags come from the dialog.
- **History**: `query.GetMessages` fetch the newest 60 messages, reverse to
  chronological order, insert Today/Yesterday/date separators, and mark the chat
  read.
- **Updates**: `tg.UpdateDispatcher` handles new messages, edits, deletes and
  reaction updates. Events go through a buffered channel; a Bubble Tea command
  waits for the next event and re-arms itself.
- **Send**: `message.Sender` sends text, optionally as a reply. Reactions use
  `messages.sendReaction` and delete uses `messages.deleteMessages` /
  `channels.deleteMessages`.
- **Uploads**: the composer's attach button (`a`) opens a path prompt; the
  bridge uploads through gotd's uploader and sends a photo for image extensions,
  otherwise a document with filename and MIME type.
- **Recent emoji**: kept locally in SQLite, prepended to the emoji panel. There
  is no recent-emoji RPC in the current layer, so this is client-side state.
- **Flood wait**: gotd's floodwait middleware is installed as an invoker, and
  dialogs have an extra retry loop. The status bar shows the countdown and the
  request resumes instead of dropping to the mock list.
- **Media**: clicking a voice/round/photo asks the bridge to fetch the message,
  download it through gotd's downloader into `TG_CACHE`, and emit
  `MediaReadyEvent`; the UI then plays it or opens the chafa viewer.
- **Fallback**: no credentials means mock chats and the original demo audio
  paths.

```
gotd/td  →  tgclient (auth, caches, typed events)  →  tea.Msg  →  UI model
                    ↑                                            │
                    └──── SendText / React / Delete / Media ─────┘
```

Current limits: archived chats and custom folders are not exposed, mute/pin
context actions are local only, QR login and uploads are missing, and read
receipts are shown as ✓✓.
