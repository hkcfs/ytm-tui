package config

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	defaultSearchResults = 25
	settingsConfName     = "settings.conf"
	legacySettingsJSON   = "settings.json"
)

// Settings holds user preferences that mirror the spec's settings menu.
type Settings struct {
	SearchResults  int    `json:"search_results"`
	UseHistory     bool   `json:"use_history"`
	ShowThumbnails bool   `json:"show_thumbnails"`
	LegacyMode     bool   `json:"legacy_mode"`
	YTDLPArgs      string `json:"ytdlp_args"`
	ExtractorArgs  string `json:"extractor_args"`
}

// Paths groups frequently used config paths.
type Paths struct {
	ConfigDir    string
	SettingsFile string
	HistoryFile  string
	PlaylistsDir string
	SocketsDir   string
}

// EnsurePaths ensures the config directory tree exists and returns the resolved paths.
func EnsurePaths() (Paths, error) {
	configDir, err := resolveConfigDir()
	if err != nil {
		return Paths{}, err
	}
	paths := Paths{
		ConfigDir:    configDir,
		SettingsFile: filepath.Join(configDir, settingsConfName),
		HistoryFile:  filepath.Join(configDir, "history.log"),
		PlaylistsDir: filepath.Join(configDir, "playlists"),
		SocketsDir:   filepath.Join(os.TempDir(), "ytm-tui"),
	}
	for _, dir := range []string{paths.ConfigDir, paths.PlaylistsDir, paths.SocketsDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return Paths{}, fmt.Errorf("create dir %s: %w", dir, err)
		}
	}
	return paths, nil
}

// LoadSettings reads the persisted settings or returns defaults if the file is missing.
func LoadSettings() (Settings, error) {
	paths, err := EnsurePaths()
	if err != nil {
		return Settings{}, err
	}
	settings, err := readConf(paths.SettingsFile)
	if err == nil {
		return normalize(settings), nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return Settings{}, err
	}
	// Fallback: attempt to migrate legacy JSON if present
	legacyPath := filepath.Join(paths.ConfigDir, legacySettingsJSON)
	legacy, lerr := readLegacyJSON(legacyPath)
	if lerr == nil {
		_ = SaveSettings(legacy) // best-effort migration
		return normalize(legacy), nil
	}
	return defaultSettings(), nil
}

// SaveSettings persists the provided settings.
func SaveSettings(s Settings) error {
	paths, err := EnsurePaths()
	if err != nil {
		return err
	}
	s = normalize(s)
	file, err := os.OpenFile(paths.SettingsFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("write settings: %w", err)
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	fmt.Fprintf(writer, "SEARCH_RESULTS=%d\n", s.SearchResults)
	fmt.Fprintf(writer, "USE_HISTORY=%d\n", boolToInt(s.UseHistory))
	fmt.Fprintf(writer, "SHOW_THUMBNAILS=%d\n", boolToInt(s.ShowThumbnails))
	fmt.Fprintf(writer, "YTM_LEGACY_MODE=%d\n", boolToInt(s.LegacyMode))
	if s.YTDLPArgs != "" {
		fmt.Fprintf(writer, "YTM_YTDLP_ARGS=%s\n", s.YTDLPArgs)
	}
	if s.ExtractorArgs != "" {
		fmt.Fprintf(writer, "YTM_YTDLP_EXTRACTOR_ARGS=%s\n", s.ExtractorArgs)
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("flush settings: %w", err)
	}
	return nil
}

func defaultSettings() Settings {
	return Settings{SearchResults: defaultSearchResults, UseHistory: true, ShowThumbnails: false}
}

func normalize(s Settings) Settings {
	if s.SearchResults <= 0 {
		s.SearchResults = defaultSearchResults
	}
	return s
}

func readConf(path string) (Settings, error) {
	file, err := os.Open(path)
	if err != nil {
		return Settings{}, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	settings := defaultSettings()
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		switch key {
		case "SEARCH_RESULTS":
			if n, err := strconv.Atoi(value); err == nil {
				settings.SearchResults = n
			}
		case "USE_HISTORY":
			settings.UseHistory = parseBool(value)
		case "SHOW_THUMBNAILS":
			settings.ShowThumbnails = parseBool(value)
		case "YTM_LEGACY_MODE":
			settings.LegacyMode = parseBool(value)
		case "YTM_YTDLP_ARGS":
			settings.YTDLPArgs = value
		case "YTM_YTDLP_EXTRACTOR_ARGS":
			settings.ExtractorArgs = value
		}
	}
	if err := scanner.Err(); err != nil {
		return Settings{}, fmt.Errorf("parse settings.conf: %w", err)
	}
	return settings, nil
}

func readLegacyJSON(path string) (Settings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Settings{}, err
	}
	var legacy Settings
	if err := json.Unmarshal(data, &legacy); err != nil {
		return Settings{}, err
	}
	return legacy, nil
}

func parseBool(value string) bool {
	value = strings.ToLower(value)
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func resolveConfigDir() (string, error) {
	if dir := os.Getenv("YTM_CONFIG_DIR"); dir != "" {
		return dir, nil
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "ytm-tui"), nil
	}
	userDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("user config dir: %w", err)
	}
	return filepath.Join(userDir, "ytm-tui"), nil
}
