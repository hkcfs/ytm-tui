package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/opencode/ytm-tui/scripts"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch the Bash+fzf TUI",
	RunE: func(cmd *cobra.Command, args []string) error {
		scriptPath, cleanup, err := prepareTUIScript()
		if err != nil {
			return err
		}
		defer cleanup()
		bashCmd := exec.Command("bash", scriptPath)
		bashCmd.Stdout = cmd.OutOrStdout()
		bashCmd.Stderr = cmd.ErrOrStderr()
		bashCmd.Stdin = cmd.InOrStdin()
		bashCmd.Env = os.Environ()
		return bashCmd.Run()
	},
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}

func prepareTUIScript() (string, func(), error) {
	if custom := os.Getenv("YTM_TUI_SCRIPT"); custom != "" {
		return custom, func() {}, nil
	}
	file, err := os.CreateTemp("", "ytm-tui-*.sh")
	if err != nil {
		return "", func() {}, fmt.Errorf("create temp script: %w", err)
	}
	if _, err := file.WriteString(scripts.TUIScript); err != nil {
		path := file.Name()
		file.Close()
		os.Remove(path)
		return "", func() {}, fmt.Errorf("write temp script: %w", err)
	}
	if err := file.Chmod(0o755); err != nil {
		path := file.Name()
		file.Close()
		os.Remove(path)
		return "", func() {}, fmt.Errorf("chmod temp script: %w", err)
	}
	path := file.Name()
	file.Close()
	cleanup := func() { _ = os.Remove(path) }
	return path, cleanup, nil
}
