package fgateway

import (
	"context"
	"net"
	"os"

	envoycache "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	xdsserver "github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"github.com/fleezesd/fgateway/internal/fgateway/controller"
	"github.com/fleezesd/fgateway/internal/fgateway/krtcollections"
	"github.com/fleezesd/fgateway/internal/fgateway/utils/krtutil"
	"github.com/fleezesd/fgateway/pkg/utils/envutil"
	"github.com/fleezesd/fgateway/pkg/utils/kubeutil"
	"github.com/solo-io/go-utils/contextutils"
	"istio.io/istio/pkg/cluster"
	istiokube "istio.io/istio/pkg/kube"
	"istio.io/istio/pkg/kube/krt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
)

func Run(ctx context.Context) error {
	SetupLogging(ctx, kubeutil.FgatewayComponentName)
	return startFgateway(ctx)
}

func createIstioClient(restConfig *rest.Config, clusterId cluster.ID) (istiokube.Client, error) {
	restCfg := istiokube.NewClientConfigForRestConfig(restConfig)
	client, err := istiokube.NewClient(restCfg, clusterId) // clusterId
	if err != nil {
		return nil, err
	}
	// watch crd change
	istiokube.EnableCrdWatcher(client)
	return client, nil
}

func startFgateway(ctx context.Context) error {
	restConfig := ctrl.GetConfigOrDie()
	// callback & ucc builder
	uniqueClientCallbacks, uccBuilder := krtcollections.NewUniquelyConnectedClients()
	// envoycache
	cache, err := startControlPlane(ctx, uniqueClientCallbacks)
	if err != nil {
		return err
	}

	opts := &controller.StartOptions{
		Cache:       cache,
		KrtDebugger: new(krt.DebugHandler),
		XdsHost: kubeutil.GetServiceFQDN(
			metav1.ObjectMeta{
				Name:      kubeutil.FgatewayServiceName,
				Namespace: kubeutil.GetPodNamespace(),
			},
		),
		XdsPort: 9000,
	}
	return startFgatewayWithConfig(ctx, restConfig, uccBuilder, opts)
}

func startControlPlane(ctx context.Context, callbacks xdsserver.Callbacks) (envoycache.SnapshotCache, error) {
	return NewControlPlane(
		ctx,
		&net.TCPAddr{IP: net.IPv4zero, Port: 9000}, // recieve any ip requests
		callbacks,
	)
}

func startFgatewayWithConfig(
	ctx context.Context,
	restConfig *rest.Config,
	uccBuilder krtcollections.UniquelyConnectedClientsBuilder,
	startOpts *controller.StartOptions,
) error {
	ctx = contextutils.WithLogger(ctx, "k8s")
	logger := contextutils.LoggerFrom(ctx)
	logger.Infof("starting %s", kubeutil.FgatewayComponentName)

	istioClient, err := createIstioClient(restConfig, cluster.ID(kubeutil.GetClusterID()))
	if err != nil {
		return err
	}
	logger.Info("creating krt collections")
	krtOpts := krtutil.NewKrtOptions(ctx.Done(), startOpts.KrtDebugger)
	// augmentedpods collection
	augmentedPods := krtcollections.NewLocalityPodsCollection(istioClient, krtOpts)
	augmentedPodsForUcc := augmentedPods
	if envutil.IsEnvTruthy("DISABLE_POD_LOCALITY_XDS") {
		augmentedPodsForUcc = nil
	}

	// ucc builder
	ucc := uccBuilder(ctx, krtOpts, augmentedPodsForUcc)
	logger.Info("initializing controller")
	// controller builder
	c, err := controller.NewControllerBuilder(ctx, controller.StartConfig{
		// todo: add extra plugin later
		Dev:           os.Getenv("LOG_LEVL") == "debug",
		StartOpts:     startOpts,
		RestConfig:    restConfig,
		Client:        istioClient,
		AugmentedPods: augmentedPods,
		UniqueClients: ucc,
		KrtOptions:    krtOpts,
	})
	if err != nil {
		logger.Error("failed initializing controller:", err)
		return err
	}
	// wait istio cache sync
	logger.Info("waiting for cache sync")
	istioClient.RunAndWait(ctx.Done())

	// 3.todo: make admin server

	// start controller
	logger.Info("starting controller")
	return c.Start(ctx)
}
