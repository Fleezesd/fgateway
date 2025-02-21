package probes

import "context"

// StartLivenessProbeServer starts a probe server listening on 8765 for requests to /healthz
// and responds with HTTP 200 with body OK
func StartLivenessProbeServer(ctx context.Context) {
	StartProbeServer(ctx, DefaultProbeServerOptions())
}
