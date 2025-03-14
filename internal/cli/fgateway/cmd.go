package fgateway

import (
	"context"
	"fmt"

	"github.com/fleezesd/fgateway/internal/fgateway"
	"github.com/fleezesd/fgateway/internal/version"
	"github.com/fleezesd/fgateway/pkg/utils/probes"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	var fgatewayVersion bool
	rootCmd := &cobra.Command{
		Use:   "fgateway",
		Short: "Runs the fgateway controller",
		RunE: func(cmd *cobra.Command, args []string) error {
			if fgatewayVersion {
				fmt.Println(version.String())
				return nil
			}
			ctx := context.Background()
			// probe server
			probes.StartLivenessProbeServer(ctx)
			if err := fgateway.Run(ctx); err != nil {
				return errors.Errorf("failed to run fgateway: %v", err)
			}
			return nil
		},
	}
	rootCmd.Flags().BoolVarP(&fgatewayVersion, "version", "v", false, "Print fgateway version")
	return rootCmd
}
