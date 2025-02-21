package main

import (
	"os"

	"github.com/fleezesd/fgateway/internal/cli/fgateway"
	"github.com/spf13/pflag"
)

func main() {
	flags := pflag.NewFlagSet("fgateway", pflag.ExitOnError)
	pflag.CommandLine = flags

	root := fgateway.NewCmd()
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
