package controller

import (
	envoycache "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"istio.io/istio/pkg/kube/krt"
)

type StartOptions struct {
	Cache       envoycache.Cache
	KrtDebugger *krt.DebugHandler

	XdsHost string
	XdsPort int32
}
