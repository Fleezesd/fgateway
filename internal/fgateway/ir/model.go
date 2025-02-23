package ir

import (
	"fmt"

	"github.com/fleezesd/fgateway/internal/fgateway/utils/hashutil"
)

const KeyDelimiter = "~"

type LocalityPod struct {
	Region  string
	Zone    string
	Subzone string
}

type UniqlyConnectedClient struct {
	Role         string
	Labels       map[string]string
	Locality     LocalityPod
	Namespace    string
	ResourceName string
}

func NewUniqlyConnectedClient(roleFromEnvoy string, ns string, labels map[string]string, locality LocalityPod) UniqlyConnectedClient {
	resourceName := roleFromEnvoy
	if ns != "" {
		snapshotKey := labeledRole(resourceName, labels)
		resourceName = fmt.Sprintf("%s%s%s", snapshotKey, KeyDelimiter, ns)
	}
	return UniqlyConnectedClient{
		Role:         roleFromEnvoy,
		Labels:       labels,
		Locality:     locality,
		Namespace:    ns,
		ResourceName: resourceName,
	}
}

func labeledRole(role string, labels map[string]string) string {
	return fmt.Sprintf("%s%s%d", role, KeyDelimiter, hashutil.HashLables(labels))
}
