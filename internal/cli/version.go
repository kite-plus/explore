package cli

import (
	"runtime"

	"github.com/spf13/cobra"

	"github.com/kite-plus/explore/internal/buildinfo"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			info := map[string]string{
				"version":  buildinfo.Version,
				"commit":   buildinfo.Commit,
				"date":     buildinfo.Date,
				"go":       runtime.Version(),
				"platform": runtime.GOOS + "/" + runtime.GOARCH,
			}
			if jsonOut(cmd) {
				return writeJSON(cmd.OutOrStdout(), info)
			}
			printf(cmd, "explore %s (%s, %s) %s %s\n", info["version"], info["commit"], info["date"], info["go"], info["platform"])
			return nil
		},
	}
}
