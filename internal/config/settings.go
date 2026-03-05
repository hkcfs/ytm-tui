package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	defaultSearchResults = 25
)

// Settings holds user preferences that mirror the spec's settings menu.
type Settings struct {
	SearchResults  int  `json:"search_results"`
	UseHistory     bool `json:"use_history"`
	ShowThumbnails bool `json:"show_thumbnails"`
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
		SettingsFile: filepath.Join(configDir, "settings.json"),
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
	data, err := os.ReadFile(paths.SettingsFile)
	if errors.Is(err, os.ErrNotExist) {
		return defaultSettings(), nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("read settings: %w", err)
	}
	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return Settings{}, fmt.Errorf("parse settings: %w", err)
	}
	return normalize(settings), nil
}

// SaveSettings persists the provided settings.
func SaveSettings(s Settings) error {
	paths, err := EnsurePaths()
	if err != nil {
		return err
	}
	s = normalize(s)
	bytes, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	if err := os.WriteFile(paths.SettingsFile, bytes, 0o644); err != nil {
		return fmt.Errorf("write settings: %w", err)
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
