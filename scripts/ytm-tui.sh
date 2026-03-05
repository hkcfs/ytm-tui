#!/usr/bin/env bash
set -Euo pipefail

APP_NAME="YTM"
CONFIG_ROOT="${YTM_CONFIG_DIR:-${XDG_CONFIG_HOME:-$HOME/.config}/ytm-tui}"
SETTINGS_FILE="$CONFIG_ROOT/settings.conf"
PLAYLIST_DIR="$CONFIG_ROOT/playlists"
HISTORY_FILE="$CONFIG_ROOT/history.log"
SOCKET_DIR="${TMPDIR:-/tmp}/ytm-tui"
MPV_SOCKET="$SOCKET_DIR/mpv.sock"

SEARCH_RESULTS=25
USE_HISTORY=1
SHOW_THUMBNAILS=0
HAVE_KITTY=0
if command -v kitty >/dev/null 2>&1; then
	HAVE_KITTY=1
fi
EXTRA_YTDLP_ARGS=()
if [[ -n "${YTM_YTDLP_ARGS:-}" ]]; then
	IFS=' ' read -r -a EXTRA_YTDLP_ARGS <<<"${YTM_YTDLP_ARGS}"
fi
LEGACY_MODE=${YTM_LEGACY_MODE:-0}
YTDLP_EXTRACTOR_ARGS=${YTM_YTDLP_EXTRACTOR_ARGS:-}
FORCED_RENDERER=${YTM_THUMB_RENDERER:-auto}
if [[ -z "$YTDLP_EXTRACTOR_ARGS" && "$LEGACY_MODE" != "1" ]]; then
	YTDLP_EXTRACTOR_ARGS="youtube:player_client=tv_embedded"
fi

THUMB_RENDERER="none"
progress_pid=""

yt_dlp() {
	command yt-dlp "${EXTRA_YTDLP_ARGS[@]}" "$@"
}

trap cleanup EXIT INT TERM

cleanup() {
	if [[ -S "$MPV_SOCKET" ]]; then
		rm -f "$MPV_SOCKET"
	fi
	tput cnorm || true
	stop_progress
}

normalize_bool() {
	local value="${1,,}"
	case "$value" in
		1|true|yes|on) echo 1 ;;
		*) echo 0 ;;
	esac
}

normalize_bool() {
	local value="${1,,}"
	case "$value" in
		1|true|yes|on) echo 1 ;;
		*) echo 0 ;;
	esac
}

swap_lines() {
	local file="$1"
	local direction="$2"
	local line_number="${3:-1}"
	[[ -f "$file" ]] || return
	mapfile -t arr <"$file"
	local size=${#arr[@]}
	(( size > 1 )) || return
	local idx=$((line_number-1))
	if (( idx < 0 || idx >= size )); then
		return
	fi
	local target=$idx
	if [[ "$direction" == "down" ]]; then
		target=$((idx+1))
	else
		target=$((idx-1))
	fi
	if (( target < 0 || target >= size )); then
		return
	fi
	local tmp="${arr[idx]}"
	arr[idx]="${arr[target]}"
	arr[target]="$tmp"
	printf '%s\n' "${arr[@]}" >"$file"
}
export -f swap_lines

ensure_prereqs() {
	for bin in fzf yt-dlp mpv jq socat tput curl; do
		if ! command -v "$bin" >/dev/null 2>&1; then
			echo "Missing dependency: $bin" >&2
			exit 1
		fi
	done
}

ensure_dirs() {
	mkdir -p "$CONFIG_ROOT" "$PLAYLIST_DIR" "$SOCKET_DIR"
}

load_settings() {
	if [[ -f "$SETTINGS_FILE" ]]; then
		while IFS='=' read -r key value; do
			case "$key" in
				SEARCH_RESULTS) SEARCH_RESULTS=${value:-25} ;;
				USE_HISTORY) USE_HISTORY=$(normalize_bool "${value:-1}") ;;
				SHOW_THUMBNAILS) SHOW_THUMBNAILS=$(normalize_bool "${value:-0}") ;;
				YTM_LEGACY_MODE) LEGACY_MODE=$(normalize_bool "${value:-0}") ;;
				YTM_YTDLP_ARGS) YTM_YTDLP_ARGS=${value} ;;
				YTM_YTDLP_EXTRACTOR_ARGS) YTDLP_EXTRACTOR_ARGS=${value} ;;
				YTM_THUMB_RENDERER) FORCED_RENDERER=${value} ;;
			esac
		done < "$SETTINGS_FILE"
	else
		save_settings
	fi
	refresh_extra_args
	if [[ -z "$YTDLP_EXTRACTOR_ARGS" && "$LEGACY_MODE" != "1" ]]; then
		YTDLP_EXTRACTOR_ARGS="youtube:player_client=tv_embedded"
	fi
	if [[ -z "$FORCED_RENDERER" ]]; then
		FORCED_RENDERER="auto"
	fi
	select_thumbnail_renderer
}

refresh_extra_args() {
	EXTRA_YTDLP_ARGS=()
	if [[ -n "${YTM_YTDLP_ARGS:-}" ]]; then
		IFS=' ' read -r -a EXTRA_YTDLP_ARGS <<<"${YTM_YTDLP_ARGS}"
	fi
}

renderer_supported() {
	local renderer="$1"
	case "$renderer" in
		kitty)
			[[ "$HAVE_KITTY" == "1" && -n "$KITTY_WINDOW_ID" ]]
			return
			;;
		wezterm)
			command -v wezterm >/dev/null 2>&1
			return
			;;
		icat)
			command -v icat >/dev/null 2>&1
			return
			;;
		img2sixel)
			command -v img2sixel >/dev/null 2>&1
			return
			;;
		chafa)
			command -v chafa >/dev/null 2>&1
			return
			;;
		viu)
			command -v viu >/dev/null 2>&1
			return
			;;
		jp2a)
			command -v jp2a >/dev/null 2>&1
			return
			;;
		img2txt)
			command -v img2txt >/dev/null 2>&1
			return
			;;
		none)
			return 0
			;;
		*)
			return 1
			;;
	esac
}

show_progress() {
	local message="$1"
	[[ -n "$progress_pid" ]] && return
	(
		local chars='|/-\\'
		local i=0
		while true; do
			local char=${chars:i%${#chars}:1}
			tput sc
			tput cup 2 0
			printf "%-50s" "$message $char"
			tput rc
			sleep 0.1
			i=$((i+1))
		done
	) &
	progress_pid=$!
}

stop_progress() {
	if [[ -n "$progress_pid" ]]; then
		kill "$progress_pid" 2>/dev/null || true
		wait "$progress_pid" 2>/dev/null || true
		progress_pid=""
		tput sc
		tput cup 2 0
		printf "%-50s" ""
		tput rc
	fi
}

select_thumbnail_renderer() {
	THUMB_RENDERER="none"
	if [[ "$SHOW_THUMBNAILS" == "1" ]]; then
		local order=()
		local forced=$(printf "%s" "$FORCED_RENDERER" | tr 'A-Z' 'a-z')
		if [[ -n "$forced" && "$forced" != "auto" ]]; then
			if [[ "$forced" == "none" ]]; then
				THUMB_RENDERER="none"
				return
			fi
			order+=("$forced")
		fi
		order+=("kitty" "wezterm" "icat" "img2sixel" "chafa" "viu" "jp2a" "img2txt")
		for renderer in "${order[@]}"; do
			[[ -z "$renderer" ]] && continue
			if renderer_supported "$renderer"; then
				THUMB_RENDERER="$renderer"
				return
			fi
		done
	fi
}

save_settings() {
	cat >"$SETTINGS_FILE" <<EOF
SEARCH_RESULTS=${SEARCH_RESULTS}
USE_HISTORY=${USE_HISTORY}
SHOW_THUMBNAILS=${SHOW_THUMBNAILS}
YTM_LEGACY_MODE=${LEGACY_MODE}
YTM_YTDLP_ARGS=${YTM_YTDLP_ARGS}
YTM_YTDLP_EXTRACTOR_ARGS=${YTDLP_EXTRACTOR_ARGS}
YTM_THUMB_RENDERER=${FORCED_RENDERER}
EOF
}

draw_frame() {
	tput civis
	tput clear
	head=$(printf "%s - Main Menu" "$APP_NAME")
	cols=$(tput cols)
	printf '\e[48;5;238;38;5;229m%*s\e[0m\n' "$cols" " $head"
	for ((i=0; i<cols; i++)); do printf '-'; done
	printf '\n'
}

footer() {
	cols=$(tput cols)
	printf '\n'
	for ((i=0; i<cols; i++)); do printf '-'; done
	printf '\nPress q to exit · arrows to navigate · Tab to multi-select\n'
	tput cnorm
}

prompt_query() {
	local default=""
	if [[ -f "$HISTORY_FILE" && $USE_HISTORY -eq 1 ]]; then
		default=$(tail -n 1 "$HISTORY_FILE" || true)
	fi
	read -rp "Search query [${default:-none}]: " query || true
	if [[ -z "$query" ]]; then
		query="$default"
	fi
	printf '%s' "$query"
}

record_history() {
	local query="$1"
	[[ $USE_HISTORY -eq 1 ]] || return 0
	[[ -n "$query" ]] || return 0
	printf '%s\n' "$query" >>"$HISTORY_FILE"
}

search_videos() {
	local query="$1"
	[[ -n "$query" ]] || return 1
	local tmp_json
	tmp_json=$(mktemp)
	local extractor_flags=()
	if [[ -n "$YTDLP_EXTRACTOR_ARGS" ]]; then
		extractor_flags=(--extractor-args "$YTDLP_EXTRACTOR_ARGS")
	fi
	show_progress "Searching YouTube..."
	if ! yt_dlp --dump-json --skip-download --no-playlist --default-search ytsearch "${extractor_flags[@]}" "ytsearch${SEARCH_RESULTS}:${query}" \
		| jq -s 'map(select(((.duration // 0) > 65) and (.webpage_url | contains("/shorts/") | not)))' >"$tmp_json"; then
		stop_progress
		rm -f "$tmp_json"
		return 1
	fi
	stop_progress
	mapfile -t RESULTS < <(jq -r 'to_entries[] | "\(.key)\t\(.value.title)\t\(.value.uploader)\t\(.value.duration_string // "??")\t\(.value.view_count // 0)\t\(.value.webpage_url)\t\(.value.thumbnail // "")"' "$tmp_json")
	rm -f "$tmp_json"
}

fzf_select() {
	local preview_script
preview_script=$(cat <<'PVS'
function human(){
	local n=$1
	if (( n > 1000000 )); then printf "%.1fM" "$(awk -v n=$n 'BEGIN{print n/1000000}')"; elif (( n > 1000 )); then printf "%.1fk" "$(awk -v n=$n 'BEGIN{print n/1000}')"; else printf "%s" "$n"; fi
}
IFS=$'\t' read -r idx title channel duration views url thumb <<<"{}"
printf "Index: %s\nTitle: %s\nChannel: %s\nDuration: %s\nViews: %s\nURL: %s\n\n" "$idx" "$title" "$channel" "$duration" "$(human "$views")" "$url"
if [[ "$SHOW_THUMBNAILS" == "1" && -n "$thumb" && "$THUMB_RENDERER" != "none" ]]; then
	cache="${TMPDIR:-/tmp}/ytm-thumb-$idx.jpg"
	if [[ ! -f "$cache" ]]; then
		curl -sL "$thumb" -o "$cache"
	fi
	case "$THUMB_RENDERER" in
		kitty)
			kitty +kitten icat --place=40x20@0x0 "$cache" 2>/dev/null
			printf '\n'
			;;
		wezterm)
			wezterm imgcat --height 20 --width 40 "$cache" 2>/dev/null || true
			printf '\n'
			;;
		icat)
			icat "$cache" 2>/dev/null || true
			printf '\n'
			;;
		img2sixel)
			img2sixel "$cache" 2>/dev/null || true
			printf '\n'
			;;
		chafa)
			chafa --size=40x20 "$cache"
			printf '\n'
			;;
		viu)
			viu -t 20 -w 40 "$cache" 2>/dev/null
			printf '\n'
			;;
		jp2a)
			jp2a --width=40 --height=20 "$cache"
			printf '\n'
			;;
		img2txt)
			img2txt -W 40 "$cache"
			printf '\n'
			;;
		esac
else
	printf "(Thumbnails disabled or renderer unavailable)\n"
fi
PVS
)
	local selection_file
	selection_file=$(mktemp)
	printf '%s\n' "${RESULTS[@]}" | \
	fzf --multi --prompt="ytm search > " --bind 'esc:abort' \
		--preview "SHOW_THUMBNAILS=${SHOW_THUMBNAILS} HAVE_KITTY=${HAVE_KITTY} THUMB_RENDERER=${THUMB_RENDERER} TMPDIR=${TMPDIR:-/tmp} KITTY_WINDOW_ID=${KITTY_WINDOW_ID:-} bash -c '$preview_script'" \
		--preview-window=right,60% >"$selection_file"
	mapfile -t SELECTION <"$selection_file"
	rm -f "$selection_file"
}

select_format() {
	local url="$1"
	local extractor_flags=()
	if [[ -n "$YTDLP_EXTRACTOR_ARGS" ]]; then
		extractor_flags=(--extractor-args "$YTDLP_EXTRACTOR_ARGS")
	fi
	show_progress "Fetching formats..."
	if ! mapfile -t FORMATS < <(yt_dlp --dump-json --skip-download "${extractor_flags[@]}" "$url" | jq -r '.formats[] | select(.vcodec == "none" and .acodec != "none") | "\(.format_id)\t\(.ext)\t\(.tbr // 0)kbps"'); then
		stop_progress
		return
	fi
	stop_progress
	if [[ ${#FORMATS[@]} -eq 0 ]]; then
		FORMAT_ID="bestaudio"
		return
	fi
	FORMAT_ID=$(printf '%s\n' "${FORMATS[@]}" | fzf --prompt="audio format > " --with-nth=1 --delimiter='\t' | cut -f1)
	FORMAT_ID=${FORMAT_ID:-bestaudio}
}

launch_mpv() {
	[[ -S "$MPV_SOCKET" ]] && rm -f "$MPV_SOCKET"
	mpv --no-video --input-ipc-server="$MPV_SOCKET" --ytdl-format="${FORMAT_ID:-bestaudio}" "$@" &
	MPV_PID=$!
	sleep 1
}

mpv_cmd() {
	local payload="$1"
	socat - "$MPV_SOCKET" <<<"$payload" >/dev/null 2>&1 || true
}

mpv_query() {
	local payload="$1"
	socat - "$MPV_SOCKET" <<<"$payload" 2>/dev/null
}

mpv_cycle_pause() {
	mpv_cmd '{"command":["cycle","pause"]}'
}

mpv_seek() {
	local delta="$1"
	mpv_cmd '{"command":["seek",'"$delta"',"relative"]}'
}

mpv_next(){ mpv_cmd '{"command":["playlist-next","force"]}'; }
mpv_prev(){ mpv_cmd '{"command":["playlist-prev","force"]}'; }

playback_loop() {
	while true; do
		if ! IFS= read -rsn1 key; then
			break
		fi
		case "$key" in
			'q') mpv_cmd '{"command":["quit"]}'; break ;;
			'p'|' ') mpv_cycle_pause ;;
			'>') mpv_next ;;
			'<') mpv_prev ;;
			$'\x1b') read -rsn2 -t 0.001 rest || true; case "$rest" in
				[C]) mpv_seek 10 ;;
				[D]) mpv_seek -10 ;;
			esac ;;
		esac
		draw_playback_status
	done
}

draw_playback_status() {
	local title channel time_pos duration state
	title=$(mpv_query '{"command":["get_property","media-title"]}' | jq -r '.data // ""') || true
	state=$(mpv_query '{"command":["get_property","pause"]}' | jq -r '.data // false') || true
	time_pos=$(mpv_query '{"command":["get_property","time-pos"]}' | jq -r '.data // 0' ) || true
	duration=$(mpv_query '{"command":["get_property","duration"]}' | jq -r '.data // 0' ) || true
	[[ -z "$title" ]] && title="Loading..."
	local status="Playing"
	[[ "$state" == "true" ]] && status="Paused"
	local progress=""
	if (( $(printf '%.0f' "$duration") > 0 )); then
		local percent=$(awk -v t="$time_pos" -v d="$duration" 'BEGIN{ if(d==0) print 0; else print int((t/d)*30) }')
		if (( percent < 0 )); then percent=0; fi
		if (( percent > 30 )); then percent=30; fi
		local fills=""
		local gaps=""
		if (( percent > 0 )); then
			fills=$(printf '#%.0s' $(seq 1 "$percent"))
		fi
		local remaining=$((30-percent))
		if (( remaining > 0 )); then
			gaps=$(printf '.%.0s' $(seq 1 "$remaining"))
		fi
		progress=$(printf '[%s%s]' "$fills" "$gaps")
	fi
	tput cup 1 0
	printf '\e[2K\rNow %s: %s\n%s\n' "$status" "$title" "$progress"
}

search_and_select() {
	draw_frame
	query=$(prompt_query)
	[[ -n "$query" ]] || return 1
	record_history "$query"
	search_videos "$query" || return 1
	if [[ ${#RESULTS[@]} -eq 0 ]]; then
		printf "No matches.\n"
		return 1
	fi
	fzf_select || return 1
	[[ ${#SELECTION[@]} -gt 0 ]] || return 1
	return 0
}

search_flow() {
	if ! search_and_select; then
		return
	fi
	local urls=()
	for line in "${SELECTION[@]}"; do
		IFS=$'\t' read -r idx title channel duration views url thumb <<<"$line"
		urls+=("$url")
		LAST_URL="$url"
	done
	select_format "${LAST_URL}"
	launch_mpv "${urls[@]}"
	draw_playback_status
	playback_loop
}

playlist_menu() {
	local options=("Create Playlist" "Edit Playlist" "Delete Playlist" "Play Playlist" "Back")
	while true; do
		choice=$(printf '%s\n' "${options[@]}" | fzf --prompt="playlists > " --height=40%) || return
		case "$choice" in
			"Create Playlist") create_playlist ;;
			"Edit Playlist") edit_playlist ;;
			"Delete Playlist") delete_playlist ;;
			"Play Playlist") play_playlist ;;
			*) return ;;
		esac
	done
}

create_playlist() {
	read -rp "Playlist name: " name || return
	[[ -n "$name" ]] || return
	printf '' >"$PLAYLIST_DIR/$name.list"
}

list_playlists() {
	mapfile -t PLAYLISTS < <(find "$PLAYLIST_DIR" -maxdepth 1 -type f -name '*.list' -printf '%f\n')
}

edit_playlist() {
	list_playlists
	if [[ ${#PLAYLISTS[@]} -eq 0 ]]; then
		printf "No playlists yet. Create one first.\n"
		sleep 1
		return 0
	fi
	local file=$(printf '%s\n' "${PLAYLISTS[@]}" | fzf --prompt="select playlist > ") || return
	file="$PLAYLIST_DIR/$file"
	local actions=("Add Videos" "Delete Videos" "Reorder" "Back")
	choice=$(printf '%s\n' "${actions[@]}" | fzf --prompt="edit > ") || return
	case "$choice" in
		"Add Videos") search_flow_add "$file" ;;
		"Delete Videos") delete_videos "$file" ;;
		"Reorder") reorder_videos "$file" ;;
	esac
}


search_flow_add() {
	local file="$1"
	if ! search_and_select; then
		return
	fi
	for line in "${SELECTION[@]}"; do
		IFS=$'\t' read -r idx title channel duration views url thumb <<<"$line"
		printf '%s | %s\n' "$url" "$title" >>"$file"
	done
}

delete_videos() {
	local file="$1"
	mapfile -t entries <"$file"
	if [[ ${#entries[@]} -eq 0 ]]; then
		printf "Playlist is empty.\n"
		sleep 1
		return 0
	fi
	mapfile -t to_remove < <(printf '%s\n' "${entries[@]}" | nl -ba | fzf --multi --prompt="delete > " | cut -f2-)
	[[ ${#to_remove[@]} -gt 0 ]] || return
	: >"$file"
	for line in "${entries[@]}"; do
		local keep=1
		for rem in "${to_remove[@]}"; do
			[[ "$line" == "$rem" ]] && keep=0 && break
		done
		(( keep )) && printf '%s\n' "$line" >>"$file"
	done
}

reorder_videos() {
	local file="$1"
	mapfile -t entries <"$file"
	if [[ ${#entries[@]} -eq 0 ]]; then
		printf "Playlist is empty.\n"
		sleep 1
		return 0
	fi
	local temp
	temp=$(mktemp)
	printf '%s\n' "${entries[@]}" >"$temp"
	if fzf \
		--prompt="reorder (Alt-↑/↓) > " \
		--header="Alt-↑/↓ swap · Enter saves · Esc cancels" \
		--bind "alt-k:execute-silent(bash -c 'swap_lines \"\$@\"' _ '$temp' up {n})+reload(nl -ba '$temp')" \
		--bind "alt-j:execute-silent(bash -c 'swap_lines \"\$@\"' _ '$temp' down {n})+reload(nl -ba '$temp')" \
		< <(nl -ba "$temp"); then
		cp "$temp" "$file"
	fi
	rm -f "$temp"
}

delete_playlist() {
	list_playlists
	if [[ ${#PLAYLISTS[@]} -eq 0 ]]; then
		printf "No playlists to delete.\n"
		sleep 1
		return 0
	fi
	local file=$(printf '%s\n' "${PLAYLISTS[@]}" | fzf --prompt="delete playlist > ") || return
	rm -f "$PLAYLIST_DIR/$file"
}

play_playlist() {
	list_playlists
	if [[ ${#PLAYLISTS[@]} -eq 0 ]]; then
		printf "No playlists yet. Create one first.\n"
		sleep 1
		return 0
	fi
	local file=$(printf '%s\n' "${PLAYLISTS[@]}" | fzf --prompt="play playlist > ") || return
	mapfile -t urls < <(cut -d'|' -f1 "$PLAYLIST_DIR/$file")
	if [[ ${#urls[@]} -eq 0 ]]; then
		printf "Playlist %s is empty.\n" "$file"
		sleep 1
		return 0
	fi
	FORMAT_ID=bestaudio
	launch_mpv "${urls[@]}"
	draw_playback_status
	playback_loop
}

settings_menu() {
	while true; do
		choice=$(printf 'SEARCH_RESULTS (%s)\nUSE_HISTORY (%s)\nSHOW_THUMBNAILS (%s)\nYTM_LEGACY_MODE (%s)\nYTM_YTDLP_ARGS (%s)\nYTM_YTDLP_EXTRACTOR_ARGS (%s)\nYTM_THUMB_RENDERER (%s)\nBack\n' "$SEARCH_RESULTS" "$USE_HISTORY" "$SHOW_THUMBNAILS" "$LEGACY_MODE" "${YTM_YTDLP_ARGS:-unset}" "${YTDLP_EXTRACTOR_ARGS:-default}" "${FORCED_RENDERER:-auto}" | fzf --prompt="settings > ") || return
		case "$choice" in
			SEARCH_RESULTS*) read -rp "Results count: " SEARCH_RESULTS ;;
			USE_HISTORY*) USE_HISTORY=$((1-USE_HISTORY)) ;;
			SHOW_THUMBNAILS*) SHOW_THUMBNAILS=$((1-SHOW_THUMBNAILS)) ;;
			YTM_LEGACY_MODE*) LEGACY_MODE=$((1-LEGACY_MODE)) ;;
			YTM_YTDLP_ARGS*) read -rp "Extra yt-dlp args: " input && YTM_YTDLP_ARGS="$input" ;;
			YTM_YTDLP_EXTRACTOR_ARGS*) read -rp "Extractor args (blank for default): " input && YTDLP_EXTRACTOR_ARGS="$input" ;;
			YTM_THUMB_RENDERER*) read -rp "Preferred previewer (kitty/wezterm/icat/img2sixel/chafa/viu/jp2a/img2txt/none/auto): " input && FORCED_RENDERER="$input" ;;
			*) save_settings; return ;;
		esac
		refresh_extra_args
		if [[ -z "$YTDLP_EXTRACTOR_ARGS" && "$LEGACY_MODE" != "1" ]]; then
			YTDLP_EXTRACTOR_ARGS="youtube:player_client=tv_embedded"
		fi
		if [[ "$FORCED_RENDERER" == "" ]]; then
			FORCED_RENDERER="auto"
		fi
		select_thumbnail_renderer
		save_settings
	done
}

main_menu() {
	ensure_prereqs
	ensure_dirs
	load_settings
	while true; do
		draw_frame
		choice=$(printf 'Search YouTube\nPlaylists\nSettings\nQuit\n' | fzf --prompt="ytm > " --height=40%) || break
		case "$choice" in
			"Search YouTube") search_flow ;;
			"Playlists") playlist_menu ;;
			"Settings") settings_menu ;;
			*) break ;;
		esac
		footer
	done
}

main_menu
