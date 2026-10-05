// Command explore runs the Explore API, the feed worker and the feed checker.
package main

import (
	"fmt"
	"os"
	// The runtime image has no zone database, and readers' time zones
	// decide the recommended stream's days.
	_ "time/tzdata"

	"github.com/kite-plus/explore/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "explore:", err)
		os.Exit(1)
	}
}
