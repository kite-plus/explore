// Package cli implements the explore command line interface.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/kite-plus/explore/internal/config"
)

// Execute runs the root command. Interrupts cancel the command's context so
// long-running commands can shut down cleanly.
func Execute() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return newRootCmd().ExecuteContext(ctx)
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "explore",
		Short: "Discover what people publish",
		Long: "Explore gathers the public feeds of independent blogs and links\n" +
			"readers back to the source site. It keeps no post content.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().BoolP("json", "j", false, "emit machine readable output")

	root.AddCommand(
		newVersionCmd(),
		newCheckCmd(),
		newSurveyCmd(),
		newServeCmd(),
		newWorkerCmd(),
		newMigrateCmd(),
	)
	return root
}

func loadConfig() (config.Config, error) {
	return config.Load(os.Getenv)
}

func jsonOut(cmd *cobra.Command) bool {
	v, _ := cmd.Flags().GetBool("json")
	return v
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

func printf(cmd *cobra.Command, format string, args ...any) {
	// A failed write to stdout is not something a command can react to.
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), format, args...)
}
