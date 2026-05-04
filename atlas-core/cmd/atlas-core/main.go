package main

import (
	"fmt"
	"os"

	"github.com/anomalyco/atlas-core/internal/app"
	"github.com/anomalyco/atlas-core/internal/envutil"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--healthcheck" {
		readyFile := envutil.OrDefault("ATLAS_READY_FILE", "/var/lib/atlas-core/.ready")
		if _, err := os.Stat(readyFile); err != nil {
			fmt.Fprintf(os.Stderr, "healthcheck failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	a, err := app.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "startup error: %v\n", err)
		os.Exit(1)
	}

	a.Logger.Info("main", "Atlas Core running, press Ctrl+C to stop")
	a.WaitForShutdown()
}
