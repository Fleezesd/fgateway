package krtcollections

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	envoy_config_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_service_discovery_v3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/fleezesd/fgateway/internal/fgateway/ir"
	"github.com/fleezesd/fgateway/internal/fgateway/xds"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/structpb"
	"istio.io/istio/pkg/kube/krt"
	"k8s.io/apimachinery/pkg/types"
)

type callbacksCollection struct {
	logger          *zap.Logger
	augmentedPods   krt.Collection[LocalityPod]
	clients         map[int64]ConnectedClient
	uniqClientCount map[string]uint64
	uniqClients     map[string]ir.UniqlyConnectedClient
	stateLock       sync.RWMutex

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

// handle stream close and cleanup
func (o *callbacksCollection) OnStreamClosed(streamId int64) {
	ucc := o.cleanup(streamId)
	if lo.IsNotNil(ucc) {
		// notify who need this collection componentes and trigger re flush computatio
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

// handle stream request
func (o *callbacksCollection) OnStreamRequest(streamId int64, r *envoy_service_discovery_v3.DiscoveryRequest) error {
	uccResourceName, isNew, err := o.add(streamId, r)
	if err != nil {
		o.logger.Debug("error processing xds client", zap.Error(err))
		return err
	}
	if uccResourceName != "" {
		nodeMetadata := r.GetNode().GetMetadata()
		if lo.IsNil(nodeMetadata) {
			nodeMetadata = &structpb.Struct{}
		}
		if lo.IsNil(nodeMetadata.GetFields()) {
			nodeMetadata.Fields = make(map[string]*structpb.Value)
		}

		o.logger.Debug("augmenting role in node metadata", zap.String("resourceName", uccResourceName))
		// set rolekey resourceName
		nodeMetadata.GetFields()[xds.RoleKey] = structpb.NewStringValue(uccResourceName)
		r.GetNode().Metadata = nodeMetadata

		if isNew {
			// trigger re computation
			o.trigger.TriggerRecomputation()
		}
	}
	return nil
}

func (o *callbacksCollection) add(streamId int64, r *envoy_service_discovery_v3.DiscoveryRequest) (string, bool, error) {
	// stream request core logic
	var pod *LocalityPod
	usePod := o.augmentedPods != nil
	if usePod && r.GetNode() != nil {
		podRef := getRef(r.GetNode())
		resourceName := krt.Named{Name: podRef.Name, Namespace: podRef.Namespace}.ResourceName()
		k := krt.Key[LocalityPod](resourceName)
		pod = o.augmentedPods.GetKey(string(k))
	}
	addedNew := false
	// lock for update resource
	o.stateLock.Lock()
	defer o.stateLock.Unlock()
	cc, ok := o.clients[streamId]
	if !ok {
		var locality ir.LocalityPod
		var ns string
		var labels map[string]string
		if usePod {
			if lo.IsNil(pod) {
				// we need to use the pod locality info, so it's an error if we can't get the pod
				return "", false, fmt.Errorf("pod not found for node %v", r.GetNode())
			} else {
				locality = pod.Locality
				ns = pod.Namespace
				labels = pod.AugmentedLabels
			}
		}
		role := GetRoleFromRequest(r)
		o.logger.Debug("adding xds client", zap.Any("locality", locality), zap.String("ns", ns), zap.Any("labels", labels), zap.String("role", role))

		// update cc & ucc
		ucc := ir.NewUniqlyConnectedClient(role, ns, labels, locality)
		cc = NewConnectedClient(ucc.ResourceName)
		o.clients[streamId] = cc

		currentUnique := o.uniqClientCount[ucc.ResourceName]
		if currentUnique == 0 {
			o.uniqClients[ucc.ResourceName] = ucc
			addedNew = true
		}
		o.uniqClientCount[ucc.ResourceName] += 1
	}
	return cc.uniqueClientName, addedNew, nil
}

// OnFetchRequest
func (o *callbacksCollection) OnFetchRequest(_ context.Context, r *envoy_service_discovery_v3.DiscoveryRequest) error {
	// nothing special to do in a fetch request, as we don't need to maintain state
	if lo.IsNil(o.augmentedPods) {
		return nil
	}

	var pod *LocalityPod
	if r.GetNode() != nil {
		podRef := getRef(r.GetNode())
		// search pod key from krt cache
		resourceName := krt.Named{Name: podRef.Name, Namespace: podRef.Namespace}.ResourceName()
		k := krt.Key[LocalityPod](resourceName)
		// get pod from augmentedPods
		pod = o.augmentedPods.GetKey(string(k))
		// make uniqly conntected client
		ucc := ir.NewUniqlyConnectedClient(GetRoleFromRequest(r), pod.Namespace, pod.AugmentedLabels, pod.Locality)

		nodeMetadata := r.GetNode().GetMetadata()
		if lo.IsNil(nodeMetadata) {
			nodeMetadata = &structpb.Struct{}
		}
		if lo.IsNil(nodeMetadata.GetFields()) {
			nodeMetadata.Fields = make(map[string]*structpb.Value)
		}

		o.logger.Debug("augmenting role in node metadata", zap.String("resourceName", ucc.ResourceName))
		// set rolekey resourceName
		nodeMetadata.GetFields()[xds.RoleKey] = structpb.NewStringValue(ucc.ResourceName)
		r.GetNode().Metadata = nodeMetadata
	} else {
		return errors.New("get node error")
	}
	return nil
}

func getRef(node *envoy_config_core_v3.Node) types.NamespacedName {
	nns := node.GetId()
	split := strings.SplitN(nns, ".", 2)
	if len(split) != 2 {
		return types.NamespacedName{}
	}
	return types.NamespacedName{
		Name:      split[0],
		Namespace: split[1],
	}
}

/*
# Envoy config example
node:
  id: "gateway-proxy-5d4f9b7f-xyz.default"  # POD_NAME.NAMESPACE
  cluster: "cluster1"
  metadata:
    # ... other metadata
*/
