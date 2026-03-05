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

## Specification Reference

The project implements the full "YTM TUI" spec:

- **Core tech:** Bash TUI (`scripts/ytm-tui.sh`) orchestrates `fzf`, `yt-dlp`, `mpv`, `socat`, `jq`, `tput`, and optional Kitty graphics. The Go CLI (Cobra) is a thin wrapper exposing both CLI + TUI commands.
- **Search:** `ytm search` and the TUI main menu offer `fzf`-driven search, exclude Shorts, and honor history toggles stored in `settings.conf`/`history.log`.
- **Playback:** Selections enqueue into `mpv` via IPC; the TUI shows track metadata, status, and progress with key bindings (`p`, `>`, `<`, arrows, `q`). Audio formats are selectable through `fzf`, defaulting to best available when skipped.
- **Persistent frame:** The Bash script draws header/footer panes with `tput`, keeps `fzf` constrained to the content pane, and updates the side panel with thumbnails/details or playback info.
- **Playlists:** Nested `fzf` menus handle create/edit/delete/play flows using plain-text `playlists/*.list` files, including multi-add, multi-delete, and reordering (Alt-↑/↓ bindings).
- **Settings page:** Accessible from the main menu, lets users tweak `SEARCH_RESULTS`, `USE_HISTORY`, and `SHOW_THUMBNAILS`, persisted instantly.
- **Command-line integration:** Cobra CLI offers `ytm search`, `ytm tui`, standard `--help`, plus `--limit`, `--play`, `--format`, `--no-history`, `--no-fzf` flags mirroring the TUI behavior.
- **References honored:** Layout, thumbnail previews, IPC control, and search mechanics mirror `ytfzf` inspiration while incorporating mpv IPC docs and Bash best practices.

## Runtime Dependencies

Install these on the host (Docker image already includes them):

- `fzf`, `yt-dlp`, `mpv`, `socat`, `jq`, `curl`, `tput`, `bash`
- Kitty terminal (optional) for thumbnails
- Keep `yt-dlp` current (`yt-dlp -U`). The Docker image fetches the latest release binary automatically; host installs should be updated manually.

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

### CLI command reference

| Command | Description | Notable flags |
|---------|-------------|---------------|
| `ytm search [query]` | Fetches YouTube results via `yt-dlp`, optionally pipes through `fzf`, and prints or plays selections. Works non-interactively when `--no-fzf` is set. | `-l/--limit`, `--play`, `--format`, `--no-history`, `--no-fzf` |
| `ytm tui` | Launches the Bash/fzf TUI (`scripts/ytm-tui.sh`) inside the current environment. In Docker this script is pre-copied into `/usr/local/share/ytm/`. | Honors env vars such as `YTM_CONFIG_DIR`, `YTM_YTDLP_ARGS`, `KITTY_WINDOW_ID` |
| `ytm --help` | Shows the Cobra command tree, including any additional subcommands you may add later. | `-v/--verbose` toggles extra logging |

## TUI quick reference

- **Main menu:** Search / Playlists / Settings / Quit.
- **Search pane:** Uses `fzf` with preview on the right; `Tab` multi-selects, `Enter` confirms, `Esc` backs out.
- **Playback controls:** `p` or `Space` toggles pause, `>` next, `<` previous, `→/←` seek ±10s, `q` quits player.
- **Playlists:** Nested `fzf` menus for add-from-search, delete entries, reorder (Alt+↑/↓), play sequentially.
- **Settings:** interactive toggles for result count, history, thumbnails.

All state lives under `~/.config/ytm-tui/` (override via `YTM_CONFIG_DIR`). Files include `settings.conf`, `history.log`, and `playlists/*.list` (`URL | Title` per line).

### Data & Storage Layout

```
~/.config/ytm-tui (or $YTM_CONFIG_DIR)
├── settings.conf        # SEARCH_RESULTS / USE_HISTORY / SHOW_THUMBNAILS knobs
├── history.log          # One search query per line (newest at bottom)
└── playlists/
    ├── chill.list       # Plain text playlist, `URL | Title` each line
    └── commute.list
```

- **Playlists:** Managed entirely by the TUI playlist menu. Each `.list` file can be edited manually; the format is simple (`https://youtu... | Track Title`). The TUI supports create/edit/delete/reorder/multi-add operations that directly mutate these files.
- **Config overrides:** Set `YTM_CONFIG_DIR=/path/to/custom-dir` before launching the CLI or TUI to relocate the whole tree (useful for Docker bind-mounts).
- **History:** Respecting `USE_HISTORY`, the CLI/TUI append successful queries to `history.log`. Disable via Settings or pass `--no-history` on `ytm search`.
- **Temporary sockets:** MPV IPC sockets live under `/tmp/ytm-tui/` (or the platform temp dir) and are cleared automatically on exit.

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

# Minimal CLI-only smoke test (Alpine image)
./minimum-build.sh "milk-v duo"
```

> Note: audio inside Docker often needs PulseAudio/ALSA passthrough; adjust flags for your setup (e.g., `-e PULSE_SERVER`).

## Development flow

```bash
make build          # go build -o bin/ytm ./cmd/ytm
make run            # run the binary locally (default --help)
make lint           # gofmt + go vet (basic hygiene)
make docker-build   # ship the container image
./build-static-bin.sh   # produce dist/ytm-linux-amd64.tar.gz via Docker
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
- `YTM_YTDLP_ARGS` – extra flags appended to every `yt-dlp` call (space-split, so prefer `--flag=value` when arguments contain spaces).
- `YTM_YTDLP_EXTRACTOR_ARGS` – override the default `youtube:player_client=tv_embedded` extractor args when YouTube requires a different client or PO token.

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

## CI / Releases

- `build-static-bin.sh` powers the release packaging: it runs inside a Dockerized Go toolchain, emits a static binary, bundles `ytm-tui.sh`, and drops the compressed artifact plus `.sha256` into `dist/`.
- `.github/workflows/release.yml` runs automatically on tags matching `v*`, invokes the script, and uploads the tarball/checksum to the GitHub Release page.
