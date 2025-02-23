package krtcollections

import (
	"context"
	"errors"
	"sync/atomic"

	envoy_config_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_service_discovery_v3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/fleezesd/fgateway/internal/fgateway/xds"
	"github.com/samber/lo"
)

type callbacks struct {
	collection atomic.Pointer[callbacksCollection]
}

// OnStreamClosed
func (o *callbacks) OnStreamClosed(streamId int64, node *envoy_config_core_v3.Node) {
	callbacksCollection := o.collection.Load()
	if lo.IsNil(callbacksCollection) {
		return
	}
	callbacksCollection.OnStreamClosed(streamId)
}

// OnStreamRequest
func (o *callbacks) OnStreamRequest(streamId int64, r *envoy_service_discovery_v3.DiscoveryRequest) error {
	// get role
	role := GetRoleFromRequest(r)
	// check gateway cache key if or not
	if !xds.IsKubeGatewayCacheKey(role) {
		return nil
	}
	c := o.collection.Load()
	if lo.IsNil(c) {
		return errors.New("fgateway not initialized")
	}
	return c.OnStreamRequest(streamId, r)
}

func (o *callbacks) OnFetchRequests(ctx context.Context, r *envoy_service_discovery_v3.DiscoveryRequest) error {
	role := GetRoleFromRequest(r)
	if !xds.IsKubeGatewayCacheKey(role) {
		return nil
	}
	c := o.collection.Load()
	if lo.IsNil(c) {
		return errors.New("fgateway not initialized")
	}
	return c.OnFetchRequest(ctx, r)
}

func GetRoleFromRequest(r *envoy_service_discovery_v3.DiscoveryRequest) string {
	return r.GetNode().GetMetadata().GetFields()[xds.RoleKey].GetStringValue()
}
