package krtcollections

import (
	"context"

	xdsserver "github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"github.com/fleezesd/fgateway/internal/fgateway/ir"
	"github.com/fleezesd/fgateway/internal/fgateway/utils/krtutil"
	"istio.io/istio/pkg/kube/krt"
)

type ConnectedClient struct {
	uniqueClientName string
}

func NewConnectedClient(uniqueClientName string) ConnectedClient {
	return ConnectedClient{
		uniqueClientName: uniqueClientName,
	}
}

// If augmentedPods is nil, we won't use the pod locality info, and all pods for the same gateway will receive the same config.
type UniquelyConnectedClientsBuilder func(ctx context.Context, krtOpts krtutil.KrtOptions, augmentedPods krt.Collection[LocalityPod]) krt.Collection[ir.UniqlyConnectedClient]

func NewUniquelyConnectedClients() (xdsserver.Callbacks, UniquelyConnectedClientsBuilder) {
	cb := &callbacks{}
	// make xdsserver callback
	envoycb := xdsserver.CallbackFuncs{
		StreamClosedFunc:  cb.OnStreamClosed,
		StreamRequestFunc: cb.OnStreamRequest,
		FetchRequestFunc:  cb.OnFetchRequests,
	}
	return envoycb, buildCollection(cb)
}

func buildCollection(callbacks *callbacks) UniquelyConnectedClientsBuilder {
	return func(ctx context.Context, krtOpts krtutil.KrtOptions, augmentPods krt.Collection[LocalityPod]) krt.Collection[ir.UniqlyConnectedClient] {
		trigger := krt.NewRecomputeTrigger(true) // istio krt ( declarative controller framework)
		col := &callbacksCollection{}

		callbacks.collection.Store(col)
		return krt.NewManyFromNothing(
			func(ctx krt.HandlerContext) []ir.UniqlyConnectedClient {
				trigger.MarkDependant(ctx)
				return col.getClients()
			},
			krtOpts.ApplyTo("UniqueConnectedClients")...,
		)
	}
}
