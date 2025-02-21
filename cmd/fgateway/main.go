package main

import (
	"log"

	"github.com/fleezesd/fgateway/internal/cli/fgateway"
	"github.com/spf13/pflag"
)

func main() {
	flags := pflag.NewFlagSet("fgateway", pflag.ExitOnError)
	pflag.CommandLine = flags

	root := fgateway.NewCmd()
	if err := root.Execute(); err != nil {
		log.Fatal(err)
	}
}
