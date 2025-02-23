package krtcollections

import (
	"maps"

	"github.com/fleezesd/fgateway/internal/fgateway/utils/krtutil"
	istiokube "istio.io/istio/pkg/kube"
	"istio.io/istio/pkg/kube/kclient"
	"istio.io/istio/pkg/kube/krt"
	corev1 "k8s.io/api/core/v1"
)

var (
	_ krt.ResourceNamer         = NodeMetadata{}
	_ krt.Equaler[NodeMetadata] = NodeMetadata{}
)

type NodeMetadata struct {
	name   string
	labels map[string]string
}

func NewNodeMetaCollection(istioClient istiokube.Client, krtOpts krtutil.KrtOptions) krt.Collection[NodeMetadata] {
	nodeClient := kclient.New[*corev1.Node](istioClient)
	nodeCollection := krt.WrapClient(nodeClient, krtOpts.ApplyTo("Node")...)
	return krt.NewCollection(nodeCollection, func(kctx krt.HandlerContext, node *corev1.Node) *NodeMetadata {
		return &NodeMetadata{
			name:   node.Name,
			labels: node.Labels,
		}
	})
}

func (c NodeMetadata) ResourceName() string {
	return c.name
}

func (c NodeMetadata) Equals(in NodeMetadata) bool {
	return c.name == in.name && maps.Equal(c.labels, in.labels)
}
