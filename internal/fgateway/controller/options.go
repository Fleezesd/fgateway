package controller

import (
	envoycache "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"istio.io/istio/pkg/kube/krt"
)

type SetupOptions struct {
	Cache               envoycache.SnapshotCache
	ExtraGatewayClasses []string

	krtDebugger *krt.DebugHandler
	XdsHost     string
	XdsPort     int32
}
