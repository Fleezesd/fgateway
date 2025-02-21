package fgateway

import (
	"context"
	"fmt"

	"github.com/fleezesd/fgateway/internal/fgateway"
	"github.com/fleezesd/fgateway/pkg/utils/probes"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	var fgatewayVersion bool
	rootCmd := &cobra.Command{
		Use:   "fgateway",
		Short: "Runs the fgateway controller",
		RunE: func(cmd *cobra.Command, args []string) error {
			if fgatewayVersion {
				fmt.Println("fgateway version 0.0.1") // will use version flag later
				return nil
			}
			ctx := context.Background()
			// probe server
			probes.StartLivenessProbeServer(ctx)
			if err := fgateway.Run(ctx); err != nil {
				return fmt.Errorf("failed to run fgateway: %w", err)
			}
			return nil
		},
	}
	rootCmd.Flags().BoolVarP(&fgatewayVersion, "version", "v", false, "Print fgateway version")
	return rootCmd
}
