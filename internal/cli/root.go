package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ytm",
	Short: "YouTube Music TUI + CLI helper",
	Long: `ytm wraps a Bash+fzf TUI and exposes search/build helpers written in Go.
It coordinates yt-dlp, mpv, fzf, and playlist management to deliver a
ytfzf-inspired experience with persistent panes and CLI access.`,
	SilenceUsage: true,
}

// Execute runs the root command tree.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose logging")
}
