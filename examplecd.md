```bash
yt-dlp --dump-json --skip-download --no-playlist \
  --default-search ytsearch \
  --extractor-args "${YTM_YTDLP_EXTRACTOR_ARGS:-youtube:player_client=tv_embedded}" \
  ${YTM_YTDLP_ARGS} \
  "ytsearch${LIMIT:-25}:${QUERY:-milk-v duo}"
```

Replace `LIMIT` and `QUERY` (or set env vars) to match your desired search. Add `--js-rt quickjs:/usr/bin/qjs` (or another runtime) via `YTM_YTDLP_ARGS` if YouTube requires it.
