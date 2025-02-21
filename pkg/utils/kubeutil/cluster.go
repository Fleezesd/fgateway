package kubeutil

import "os"

func GetClusterID() string {
	if clusterID := os.Getenv("CLUSTER_ID"); clusterID != "" {
		return clusterID
	}
	return "fgateway-cluster"
}
