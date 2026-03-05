package cli

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/opencode/ytm-tui/internal/config"
	"github.com/opencode/ytm-tui/internal/history"
	"github.com/opencode/ytm-tui/internal/search"
)

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search YouTube Music via yt-dlp and fzf",
	Args:  cobra.ArbitraryArgs,
	RunE:  runSearch,
}

func init() {
	rootCmd.AddCommand(searchCmd)
	searchCmd.Flags().IntP("limit", "l", 0, "number of results to request")
	searchCmd.Flags().Bool("no-history", false, "do not record this query")
	searchCmd.Flags().Bool("no-fzf", false, "print JSON results instead of launching fzf")
	searchCmd.Flags().Bool("play", false, "immediately enqueue the selected tracks in mpv")
	searchCmd.Flags().String("format", "", "yt-dlp format selector (default: bestaudio)")
}

func runSearch(cmd *cobra.Command, args []string) error {
	settings, err := config.LoadSettings()
	if err != nil {
		return err
	}
	paths, err := config.EnsurePaths()
	if err != nil {
		return err
	}
	limitFlag, _ := cmd.Flags().GetInt("limit")
	limit := settings.SearchResults
	if limitFlag > 0 {
		limit = limitFlag
	}
	query := strings.TrimSpace(strings.Join(args, " "))
	if query == "" {
		fmt.Fprint(cmd.OutOrStdout(), "Enter search query: ")
		line, _ := readLine(cmd.InOrStdin())
		query = strings.TrimSpace(line)
	}
	if query == "" {
		return errors.New("empty query")
	}
	noHistory, _ := cmd.Flags().GetBool("no-history")
	if settings.UseHistory && !noHistory {
		_ = history.Append(paths.HistoryFile, query)
	}
	options := buildSearchOptions(settings)
	spin := newSpinner(cmd.ErrOrStderr(), "Searching YouTube")
	spin.Start()
	videos, err := search.Search(query, limit, options)
	spin.Stop()
	if err != nil {
		return err
	}
	if len(videos) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No videos found (shorts are filtered out).")
		return nil
	}
	noFZF, _ := cmd.Flags().GetBool("no-fzf")
	if noFZF {
		for _, v := range videos {
			fmt.Fprintf(cmd.OutOrStdout(), "%s | %s | %s\n", v.Title, v.Channel, v.URL)
		}
		return nil
	}
	indices, err := launchFZF(videos)
	if err != nil {
		return err
	}
	if len(indices) == 0 {
		return nil
	}
	var urls []string
	for _, idx := range indices {
		if idx >= 0 && idx < len(videos) {
			urls = append(urls, videos[idx].URL)
		}
	}
	playFlag, _ := cmd.Flags().GetBool("play")
	if !playFlag {
		for _, idx := range indices {
			v := videos[idx]
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n%s\n\n", v.Title, v.URL)
		}
		return nil
	}
	formatSelector, _ := cmd.Flags().GetString("format")
	return enqueueWithMPV(cmd, urls, formatSelector)
}

func launchFZF(videos []search.Video) ([]int, error) {
	if _, err := exec.LookPath("fzf"); err != nil {
		return nil, fmt.Errorf("fzf is required: %w", err)
	}
	var buf bytes.Buffer
	for idx, v := range videos {
		fmt.Fprintf(&buf, "%d\t%s\t%s\t%s\t%d\t%s\n", idx, v.Title, v.Channel, v.Duration, v.ViewCount, v.URL)
	}
	preview := `awk -F '\t' '{printf "Index: %s\nTitle: %s\nChannel: %s\nDuration: %s\nViews: %s\nURL: %s\n", $1, $2, $3, $4, $5, $6}'`
	cmd := exec.Command("fzf",
		"--multi",
		"--ansi",
		"--prompt=ytm> ",
		"--delimiter=\t",
		"--with-nth=2,3,4",
		"--preview", preview,
		"--preview-window=right,60%",
		"--header", "Select tracks (Tab to mark, Enter to confirm)")
	cmd.Stdin = &buf
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if exitErr.ExitCode() == 130 {
				return nil, nil
			}
		}
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	var indices []int
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		idx, err := search.ParseFZFSelection(line)
		if err != nil {
			return nil, err
		}
		indices = append(indices, idx)
	}
	return indices, nil
}

func readLine(r io.Reader) (string, error) {
	reader := bufio.NewReader(r)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func enqueueWithMPV(cmd *cobra.Command, urls []string, format string) error {
	if len(urls) == 0 {
		return nil
	}
	args := []string{"--no-video", "--keep-open=no"}
	if format != "" {
		args = append(args, "--ytdl-format="+format)
	} else {
		args = append(args, "--ytdl-format=bestaudio/best")
	}
	args = append(args, urls...)
	mpvCmd := exec.Command("mpv", args...)
	mpvCmd.Stdout = cmd.OutOrStdout()
	mpvCmd.Stderr = cmd.ErrOrStderr()
	return mpvCmd.Run()
}

func buildSearchOptions(settings config.Settings) search.Options {
	opts := search.Options{
		ExtraArgs:     splitArgs(settings.YTDLPArgs),
		ExtractorArgs: settings.ExtractorArgs,
		Legacy:        settings.LegacyMode,
	}
	if env := strings.TrimSpace(os.Getenv("YTM_YTDLP_ARGS")); env != "" {
		opts.ExtraArgs = splitArgs(env)
	}
	if env := strings.TrimSpace(os.Getenv("YTM_YTDLP_EXTRACTOR_ARGS")); env != "" {
		opts.ExtractorArgs = env
	}
	if env := strings.TrimSpace(os.Getenv("YTM_LEGACY_MODE")); env != "" {
		opts.Legacy = parseBoolEnv(env)
	}
	return opts
}

func splitArgs(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	return strings.Fields(raw)
}

func parseBoolEnv(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

type spinner struct {
	writer  io.Writer
	message string
	stop    chan struct{}
	done    chan struct{}
	chars   []rune
}

func newSpinner(w io.Writer, message string) *spinner {
	if w == nil {
		return nil
	}
	return &spinner{
		writer:  w,
		message: message,
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
		chars:   []rune{'|', '/', '-', '\\'},
	}
}

func (s *spinner) Start() {
	if s == nil {
		return
	}
	go func() {
		defer close(s.done)
		idx := 0
		fmt.Fprintf(s.writer, "%s ", s.message)
		for {
			select {
			case <-s.stop:
				fmt.Fprintf(s.writer, "\r%s ✓\n", s.message)
				return
			case <-time.After(120 * time.Millisecond):
				char := s.chars[idx%len(s.chars)]
				fmt.Fprintf(s.writer, "\r%s %c", s.message, char)
				idx++
			}
		}
	}()
}

func (s *spinner) Stop() {
	if s == nil {
		return
	}
	select {
	case <-s.done:
		return
	case s.stop <- struct{}{}:
		<-s.done
	}
}
