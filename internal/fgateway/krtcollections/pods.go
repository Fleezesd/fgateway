package krtcollections

import (
	"maps"

	"github.com/fleezesd/fgateway/internal/fgateway/ir"
	"github.com/fleezesd/fgateway/internal/fgateway/utils/krtutil"
	"github.com/samber/lo"
	istiolabel "istio.io/istio/pilot/pkg/serviceregistry/util/label"
	istiokube "istio.io/istio/pkg/kube"
	"istio.io/istio/pkg/kube/kclient"
	"istio.io/istio/pkg/kube/krt"
	"istio.io/istio/pkg/slices"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

type LocalityPod struct {
	// Named is a convenience struct. It is ideal to be embedded into a type that has a name and namespace,
	// and will automatically implement the various interfaces to return the name, namespace, and a key based on these two.
	krt.Named

	Locality        ir.LocalityPod
	AugmentedLabels map[string]string
	Addresses       []string
}

func (c LocalityPod) IP() string {
	if len(c.Addresses) == 0 {
		return ""
	}
	return c.Addresses[0]
}

func (c LocalityPod) Equals(in LocalityPod) bool {
	return c.Named == in.Named &&
		c.Locality == in.Locality &&
		maps.Equal(c.AugmentedLabels, in.AugmentedLabels) &&
		slices.Equal(c.Addresses, in.Addresses)
}

// Pods collection cache
func NewLocalityPodsCollection(istioClient istiokube.Client, krtOpts krtutil.KrtOptions) krt.Collection[LocalityPod] {
	podClient := kclient.NewFiltered[*corev1.Pod](
		istioClient, kclient.Filter{
			// StripPodUnusedFields is the transform function for shared pod informers,
			// it removes unused fields from objects before they are stored in the cache to save memory.
			ObjectTransform: istiokube.StripPodUnusedFields,
		},
	)
	// WrapClient is the base entrypoint that enables the creation
	// of a collection from an API Server client.
	podCollection := krt.WrapClient(podClient, krtOpts.ApplyTo("Pods")...)
	nodeMetadataCollection := NewNodeMetaCollection(istioClient, krtOpts)
	return krt.NewCollection(podCollection, augmentPodLabels(nodeMetadataCollection))
}

func augmentPodLabels(nodeMetadataCollection krt.Collection[NodeMetadata]) func(kctx krt.HandlerContext, pod *corev1.Pod) *LocalityPod {
	return func(kctx krt.HandlerContext, pod *corev1.Pod) *LocalityPod {
		labels := maps.Clone(pod.Labels)
		if lo.IsNil(labels) {
			labels = make(map[string]string)
		}
		nodeName := pod.Spec.NodeName
		var localityPod ir.LocalityPod
		if nodeName != "" {
			maybeNode := krt.FetchOne[NodeMetadata](kctx, nodeMetadataCollection, krt.FilterObjectName(types.NamespacedName{
				Name: nodeName,
			}))
			if lo.IsNotNil(maybeNode) {
				node := maybeNode
				nodeLabels := node.labels
				localityPod = localityFromLabels(nodeLabels)
				AugmentLabels(localityPod, labels)
			}
		}
		return &LocalityPod{
			Named:           krt.NewNamed(pod),
			Locality:        localityPod,
			AugmentedLabels: labels,
			Addresses:       extractPodIPs(pod),
		}
	}
}

func localityFromLabels(labels map[string]string) ir.LocalityPod {
	return ir.LocalityPod{
		Region:  labels[istiolabel.LabelTopologyRegion],
		Zone:    labels[istiolabel.LabelTopologyZone],
		Subzone: labels[istiolabel.LabelTopologySubzone],
	}
}

func AugmentLabels(locality ir.LocalityPod, labels map[string]string) {
	// augment labels
	if locality.Region != "" {
		labels[istiolabel.LabelTopologyRegion] = locality.Region
	}
	if locality.Zone != "" {
		labels[istiolabel.LabelTopologyZone] = locality.Zone
	}
	if locality.Subzone != "" {
		labels[istiolabel.LabelTopologySubzone] = locality.Subzone
	}
}

func extractPodIPs(pod *corev1.Pod) []string {
	if len(pod.Status.PodIPs) > 0 {
		return slices.Map(pod.Status.PodIPs, func(e corev1.PodIP) string {
			return e.IP
		})
	} else if pod.Status.PodIP != "" {
		return []string{pod.Status.PodIP}
	}
	return nil
}
