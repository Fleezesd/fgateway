package kubeutil

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/network"
)

func ServiceFQDN(serviceMeta metav1.ObjectMeta) string {
	return network.GetServiceHostname(serviceMeta.Name, serviceMeta.Namespace)
}
