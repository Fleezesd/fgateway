package xds

import (
	envoy_config_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoycache "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/samber/lo"
)

var _ envoycache.NodeHash = new(nodeRoleHasher)

const (

	// RoleKey is the name of the ket in the node.metadata used to store the role
	RoleKey = "role"

	// FallbackNodeCacheKey is used to let nodes know they have a bad config
	// we assign a "fix me" snapshot for bad nodes
	FallbackNodeCacheKey = "misconfigured-node"
)

func NewNodeRoleHasher() envoycache.NodeHash {
	return &nodeRoleHasher{}
}

// nodeRoleHasher identifies a node based on the values provided in the `node.metadata.role`
type nodeRoleHasher struct{}

func (o *nodeRoleHasher) ID(node *envoy_config_core_v3.Node) string {
	if lo.IsNotNil(node.GetMetadata()) {
		roleValue := node.GetMetadata().GetFields()[RoleKey]
		if lo.IsNotNil(roleValue) {
			return roleValue.GetStringValue()
		}
	}
	return FallbackNodeCacheKey // not configured
}
