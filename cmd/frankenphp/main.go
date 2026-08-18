package main

import (
	"fmt"
	"os"

	"frankenphp-manager/internal/cli"
)

var version = "dev"

func main() {
	cli.SetVersion(version)
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
