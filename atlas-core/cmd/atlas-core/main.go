package main

import (
	"fmt"
	"os"

	"github.com/anomalyco/atlas-core/internal/app"
)

func main() {
	a, err := app.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "startup error: %v\n", err)
		os.Exit(1)
	}

	a.Logger.Info("main", "Atlas Core running, press Ctrl+C to stop")
	a.WaitForShutdown()
}
