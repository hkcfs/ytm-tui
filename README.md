# YTM TUI

Go-backed CLI plus Bash/fzf TUI for searching, queueing, and playing YouTube audio with `yt-dlp` + `mpv`. Think of it as a batteries-included, `ytfzf`-inspired workspace that exposes both `ytm search` (CLI) and `ytm tui` (full-screen) entry points.

## Highlights

- **True dual-mode:** `ytm search <query>` for quick CLI flows and `ytm tui` for the persistent-pane experience.
- **Spec-compliant stack:** `bash`, `fzf`, `yt-dlp`, `mpv`, `socat`, `jq`, and Kitty thumbnails where available.
- **Short-aware search:** Shorts are filtered automatically; history logging is opt-in/out via settings.
- **Audio control:** `mpv` IPC for pause/play, seek (±10s), next/prev, quit, and progress visualization.
- **Playlist workbench:** Create/edit/delete/play lists stored at `~/.config/ytm-tui/playlists/*.list` with mole-like nested menus and multi-select helpers.
- **Settings UI:** TUI sliders/toggles for `SEARCH_RESULTS`, `USE_HISTORY`, and `SHOW_THUMBNAILS`, persisted to `settings.conf`.
- **Kitty thumbnails:** When `SHOW_THUMBNAILS=1` and Kitty is detected, previews render via `kitty +kitten icat`.
- **Docker-ready:** Multi-stage image ships Go binary plus all runtime deps for reproducible usage.

## Runtime Dependencies

Install these on the host (Docker image already includes them):

- `fzf`, `yt-dlp`, `mpv`, `socat`, `jq`, `curl`, `tput`, `bash`
- Kitty terminal (optional) for thumbnails

## CLI usage

```bash
ytm search "ambient piano"          # launches fzf selector and prints URLs
ytm search "dj set" --play           # stream immediately through mpv
ytm search --limit 50 --no-history    # override config defaults
ytm tui                               # launch the full TUI frame
```

Flags worth knowing:

- `--limit` (`-l`) – override `SEARCH_RESULTS` for this invocation.
- `--no-history` – skip history append even if enabled globally.
- `--no-fzf` – dump raw lines instead of invoking `fzf`.
- `--play` / `--format <yt-dlp fmt>` – enqueue picks into `mpv` directly.

## TUI quick reference

- **Main menu:** Search / Playlists / Settings / Quit.
- **Search pane:** Uses `fzf` with preview on the right; `Tab` multi-selects, `Enter` confirms, `Esc` backs out.
- **Playback controls:** `p` or `Space` toggles pause, `>` next, `<` previous, `→/←` seek ±10s, `q` quits player.
- **Playlists:** Nested `fzf` menus for add-from-search, delete entries, reorder (Alt+↑/↓), play sequentially.
- **Settings:** interactive toggles for result count, history, thumbnails.

All state lives under `~/.config/ytm-tui/` (override via `YTM_CONFIG_DIR`). Files include `settings.conf`, `history.log`, and `playlists/*.list` (`URL | Title` per line).

## Docker workflow

```bash
make docker-build                       # builds ghcr.io-style image locally
make docker-run                         # launches containerized TUI

# Manual invocation
docker build -t ytm-tui .
docker run --rm -it \
  --volume $HOME/.config/ytm-tui:/root/.config/ytm-tui \
  --device /dev/snd \
  --name ytm ytm-tui tui
```

> Note: audio inside Docker often needs PulseAudio/ALSA passthrough; adjust flags for your setup (e.g., `-e PULSE_SERVER`).

## Development flow

```bash
make build          # go build -o bin/ytm ./cmd/ytm
make run            # run the binary locally (default --help)
make lint           # gofmt + go vet (basic hygiene)
make docker-build   # ship the container image
```

Project layout:

- `cmd/ytm` – Cobra entrypoint exposing CLI + TUI launcher
- `internal/config` – config dir resolution + settings persistence
- `internal/history` – search history helpers
- `internal/search` – `yt-dlp` orchestration & format helpers
- `scripts/ytm-tui.sh` – the spec-compliant Bash/fzf TUI

## Configuration knobs

Environment variables:

- `YTM_CONFIG_DIR` – override config root (default `~/.config/ytm-tui`).
- `YTM_TUI_SCRIPT` – explicit path to `ytm-tui.sh` for the launcher.

Settings file (`settings.conf`):

```
SEARCH_RESULTS=25
USE_HISTORY=1
SHOW_THUMBNAILS=0
```

## Troubleshooting

- Missing binaries? run `make doctor` (or manually ensure deps listed above).
- Kitty thumbnails not showing: ensure `SHOW_THUMBNAILS=1` in settings **and** the terminal exports `KITTY_WINDOW_ID`.
- `mpv` IPC errors: confirm `socat` and `mpv` are installed and the socket dir (`/tmp/ytm-tui`) is writable.

Enjoy the tunes! 🎧
