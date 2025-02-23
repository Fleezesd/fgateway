package xds

import (
	"strings"

	"github.com/fleezesd/fgateway/internal/fgateway/wellknown"
)

const (
	// RoleKey is the name of the ket in the node.metadata used to store the role
	RoleKey = "role"
)

func IsKubeGatewayCacheKey(key string) bool {
	return strings.HasPrefix(key, wellknown.GatewayApiProxyValue)
}
