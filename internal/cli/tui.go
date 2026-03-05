package cli

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch the Bash+fzf TUI",
	RunE: func(cmd *cobra.Command, args []string) error {
		script, err := resolveTUIScript()
		if err != nil {
			return err
		}
		bashCmd := exec.Command("bash", script)
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

func resolveTUIScript() (string, error) {
	var candidates []string
	if custom := os.Getenv("YTM_TUI_SCRIPT"); custom != "" {
		candidates = append(candidates, custom)
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(cwd, "scripts", "ytm-tui.sh"))
	}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, "..", "scripts", "ytm-tui.sh"),
			filepath.Join(exeDir, "ytm-tui.sh"),
		)
	}
	candidates = append(candidates, "/usr/local/share/ytm/ytm-tui.sh")
	for _, path := range candidates {
		if path == "" {
			continue
		}
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return filepath.Clean(path), nil
		}
	}
	return "", errors.New("could not find ytm-tui.sh; set YTM_TUI_SCRIPT")
}
