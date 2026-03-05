package search

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Video mirrors the structured search results we display.
const defaultExtractorArgs = "youtube:player_client=tv_embedded"

// Options configures how yt-dlp commands are built.
type Options struct {
	ExtraArgs     []string
	ExtractorArgs string
	Legacy        bool
}

func (opts Options) applyDefaults() Options {
	if opts.ExtractorArgs == "" && !opts.Legacy {
		opts.ExtractorArgs = defaultExtractorArgs
	}
	return opts
}

type Video struct {
	ID              string      `json:"id"`
	Title           string      `json:"title"`
	Channel         string      `json:"uploader"`
	URL             string      `json:"webpage_url"`
	Duration        string      `json:"duration_string"`
	DurationSeconds int         `json:"duration"`
	ViewCount       int64       `json:"view_count"`
	Thumbnail       string      `json:"thumbnail"`
	Thumbnails      []Thumbnail `json:"thumbnails"`
	IsShort         bool        `json:"short"`
}

// Thumbnail captures yt-dlp thumbnail metadata we care about.
type Thumbnail struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// Format describes an audio format reported by yt-dlp.
type Format struct {
	ID       string
	Ext      string
	Bitrate  string
	Note     string
	Filesize string
}

var shortsRegex = regexp.MustCompile(`/shorts/`)

// Search queries YouTube through yt-dlp and returns a filtered list of videos, omitting shorts.
func Search(query string, limit int, opts Options) ([]Video, error) {
	opts = opts.applyDefaults()
	if strings.TrimSpace(query) == "" {
		return nil, errors.New("query cannot be empty")
	}
	args := []string{
		"--dump-json",
		"--skip-download",
		"--no-playlist",
		"--default-search", "ytsearch",
	}
	if !opts.Legacy && opts.ExtractorArgs != "" {
		args = append(args, "--extractor-args", opts.ExtractorArgs)
	}
	if len(opts.ExtraArgs) > 0 {
		args = append(args, opts.ExtraArgs...)
	}
	args = append(args, fmt.Sprintf("ytsearch%d:%s", limit, query))
	cmd := exec.Command("yt-dlp", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start yt-dlp: %w", err)
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
	var videos []Video
	for scanner.Scan() {
		line := scanner.Bytes()
		var video Video
		if err := json.Unmarshal(line, &video); err != nil {
			return nil, fmt.Errorf("decode yt-dlp json: %w", err)
		}
		if video.Title == "" || video.URL == "" {
			continue
		}
		if isShort(video) {
			continue
		}
		videos = append(videos, hydrateVideo(video))
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan yt-dlp output: %w", err)
	}
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("wait for yt-dlp: %w", err)
	}
	return videos, nil
}

// Formats fetches available audio-only formats for the given URL.
func Formats(url string, opts Options) ([]Format, error) {
	opts = opts.applyDefaults()
	args := []string{"--dump-json", "--skip-download"}
	if !opts.Legacy && opts.ExtractorArgs != "" {
		args = append(args, "--extractor-args", opts.ExtractorArgs)
	}
	if len(opts.ExtraArgs) > 0 {
		args = append(args, opts.ExtraArgs...)
	}
	args = append(args, url)
	cmd := exec.Command("yt-dlp", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("yt-dlp formats: %w", err)
	}
	var payload struct {
		Formats []struct {
			FormatID       string  `json:"format_id"`
			Ext            string  `json:"ext"`
			Acodec         string  `json:"acodec"`
			Vcodec         string  `json:"vcodec"`
			TBR            float64 `json:"tbr"`
			Format         string  `json:"format"`
			Filesize       int64   `json:"filesize"`
			FilesizeApprox int64   `json:"filesize_approx"`
		}
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, fmt.Errorf("decode formats: %w", err)
	}
	var formats []Format
	for _, f := range payload.Formats {
		if f.Acodec == "none" {
			continue
		}
		if f.Vcodec != "none" {
			continue
		}
		bitrate := fmt.Sprintf("%dk", int(f.TBR))
		filesize := humanBytes(f.Filesize)
		if filesize == "" {
			filesize = humanBytes(f.FilesizeApprox)
		}
		formats = append(formats, Format{
			ID:       f.FormatID,
			Ext:      f.Ext,
			Bitrate:  bitrate,
			Note:     f.Format,
			Filesize: filesize,
		})
	}
	sort.SliceStable(formats, func(i, j int) bool { return formats[i].Bitrate > formats[j].Bitrate })
	return formats, nil
}

func hydrateVideo(v Video) Video {
	if v.Duration == "" && v.DurationSeconds > 0 {
		v.Duration = secondsToClock(v.DurationSeconds)
	}
	if len(v.Thumbnails) == 0 && v.Thumbnail != "" {
		v.Thumbnails = []Thumbnail{{URL: v.Thumbnail}}
	}
	if v.Thumbnail == "" && len(v.Thumbnails) > 0 {
		v.Thumbnail = v.Thumbnails[0].URL
	}
	return v
}

func isShort(v Video) bool {
	if v.IsShort {
		return true
	}
	if shortsRegex.MatchString(v.URL) {
		return true
	}
	if v.DurationSeconds > 0 && v.DurationSeconds <= 65 {
		return true
	}
	return false
}

func humanBytes(size int64) string {
	if size <= 0 {
		return ""
	}
	units := []string{"B", "KB", "MB", "GB"}
	value := float64(size)
	var idx int
	for value >= 1024 && idx < len(units)-1 {
		value /= 1024
		idx++
	}
	return fmt.Sprintf("%.1f%s", value, units[idx])
}

func secondsToClock(sec int) string {
	s := sec % 60
	m := (sec / 60) % 60
	h := sec / 3600
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

// ParseFZFSelection splits an fzf selection line back into the indexed slice index.
func ParseFZFSelection(line string) (int, error) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return -1, errors.New("empty selection")
	}
	idx, err := strconv.Atoi(fields[0])
	if err != nil {
		return -1, fmt.Errorf("parse index: %w", err)
	}
	return idx, nil
}
