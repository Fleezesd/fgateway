package krtcollectoions

import (
	"sync"
	"sync/atomic"

	envoy_config_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	"github.com/fleezesd/fgateway/internal/fgateway/ir"
	"github.com/samber/lo"
	"istio.io/istio/pkg/kube/krt"
)

type callbacks struct {
	collection atomic.Pointer[callbacksCollection]
}

// OnStreamClosed Handle callback when xds stream close
func (o *callbacks) OnStreamClosed(streamId int64, node *envoy_config_core_v3.Node) {
	callbacksCollection := o.collection.Load()
	if lo.IsNil(callbacksCollection) {
		return
	}
	callbacksCollection.OnStreamClosed(streamId)
}

type callbacksCollection struct {
	stateLock       sync.RWMutex
	clients         map[int64]ConnectedClient
	uniqClientCount map[string]uint64
	uniqClients     map[string]ir.UniqlyConnectedClient

	trigger *krt.RecomputeTrigger
}

func (o *callbacksCollection) getClients() []ir.UniqlyConnectedClient {
	o.stateLock.RLock()
	defer o.stateLock.RUnlock()
	clients := make([]ir.UniqlyConnectedClient, 0, len(o.uniqClients))
	for _, c := range o.uniqClients {
		clients = append(clients, c)
	}
	return clients
}

func (o *callbacksCollection) OnStreamClosed(streamId int64) {
	ucc := o.cleanup(streamId)
	if lo.IsNotNil(ucc) {
		o.trigger.TriggerRecomputation()
	}
}

func (o *callbacksCollection) cleanup(streamId int64) *ir.UniqlyConnectedClient {
	o.stateLock.Lock()
	defer o.stateLock.Unlock()

	connectedClient, ok := o.clients[streamId]
	delete(o.clients, streamId)
	if ok {
		resourceName := connectedClient.uniqueClientName
		current := o.uniqClientCount[resourceName]
		o.uniqClientCount[resourceName] -= 1
		if current == 1 {
			ucc := o.uniqClients[resourceName]
			delete(o.uniqClientCount, resourceName)
			delete(o.uniqClients, resourceName)
			return &ucc
		}
	}
	return nil
}
