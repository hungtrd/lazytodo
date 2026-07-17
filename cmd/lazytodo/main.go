package main

import (
	"fmt"
	"os"

	"github.com/hungtrd/lazytodo/internal/cli"
	"github.com/hungtrd/lazytodo/internal/ui"
)

func main() {
	root := cli.NewRootCommand(cli.Dependencies{
		RunUI: ui.Run,
		In:    os.Stdin,
		Out:   os.Stdout,
		Err:   os.Stderr,
	})
	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
