package main

import (
	"fmt"
	"os"

	"github.com/AngheloAlva/timer/internal/cli"
)

func main() {
	root := cli.NewRootCmd()
	root.SetOut(os.Stdout)
	root.SetErr(os.Stderr)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
