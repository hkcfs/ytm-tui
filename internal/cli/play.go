package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/opencode/ytm-tui/internal/config"
)

var playCmd = &cobra.Command{
	Use:   "play [urls...]",
	Short: "Play URLs or playlists via mpv",
	Args:  cobra.ArbitraryArgs,
	RunE:  runPlay,
}

func init() {
	rootCmd.AddCommand(playCmd)
	playCmd.Flags().StringP("playlist", "p", "", "Playlist name or path to play")
	playCmd.Flags().String("format", "", "yt-dlp format selector (default: bestaudio)")
}

func runPlay(cmd *cobra.Command, args []string) error {
	paths, err := config.EnsurePaths()
	if err != nil {
		return err
	}
	playlistName, _ := cmd.Flags().GetString("playlist")
	formatSelector, _ := cmd.Flags().GetString("format")
	var urls []string
	for _, arg := range args {
		url, err := normalizeInputToURL(arg)
		if err != nil {
			return err
		}
		urls = append(urls, url)
	}
	if playlistName != "" {
		playlistURLs, err := loadPlaylistFile(paths, playlistName)
		if err != nil {
			if url, normalizeErr := normalizeInputToURL(playlistName); normalizeErr == nil {
				urls = append(urls, url)
			} else {
				return err
			}
		} else {
			urls = append(urls, playlistURLs...)
		}
	}
	if len(urls) == 0 {
		return errors.New("provide URLs or --playlist")
	}
	return enqueueWithMPV(cmd, urls, formatSelector)
}

func loadPlaylistFile(paths config.Paths, name string) ([]string, error) {
	path, err := resolvePlaylistPath(paths, name)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open playlist: %w", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	var urls []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		url := strings.TrimSpace(parts[0])
		if url != "" {
			urls = append(urls, url)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read playlist: %w", err)
	}
	if len(urls) == 0 {
		return nil, fmt.Errorf("playlist %s is empty", path)
	}
	return urls, nil
}

func resolvePlaylistPath(paths config.Paths, name string) (string, error) {
	candidates := []string{name}
	if !filepath.IsAbs(name) {
		candidates = append(candidates, filepath.Join(paths.PlaylistsDir, name))
		if !strings.HasSuffix(name, ".list") {
			candidates = append(candidates, filepath.Join(paths.PlaylistsDir, name+".list"))
		}
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("playlist not found: %s", name)
}

var videoIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

var playlistPrefixes = []string{"PL", "UU", "OL", "LL", "FL", "RD"}

func normalizeInputToURL(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", errors.New("empty input")
	}
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return trimmed, nil
	}
	if strings.HasPrefix(lower, "youtu.be/") {
		return "https://" + trimmed, nil
	}
	if strings.HasPrefix(lower, "www.youtube.com") || strings.HasPrefix(lower, "youtube.com") {
		return "https://" + trimmed, nil
	}
	if videoIDPattern.MatchString(trimmed) {
		return "https://www.youtube.com/watch?v=" + trimmed, nil
	}
	upper := strings.ToUpper(trimmed)
	for _, prefix := range playlistPrefixes {
		if strings.HasPrefix(upper, prefix) {
			return "https://www.youtube.com/playlist?list=" + trimmed, nil
		}
	}
	return "", fmt.Errorf("unable to interpret input: %s", input)
}
