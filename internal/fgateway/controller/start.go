package controller

import (
	"context"

	envoycache "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/fleezesd/fgateway/internal/fgateway/deployer"
	"github.com/fleezesd/fgateway/internal/fgateway/extension/settings"
	"github.com/fleezesd/fgateway/internal/fgateway/ir"
	"github.com/fleezesd/fgateway/internal/fgateway/krtcollections"
	"github.com/fleezesd/fgateway/internal/fgateway/utils/krtutil"
	"github.com/fleezesd/fgateway/internal/fgateway/wellknown"
	"github.com/solo-io/go-utils/contextutils"
	"go.uber.org/zap"
	istiokube "istio.io/istio/pkg/kube"
	"istio.io/istio/pkg/kube/krt"
	istiolog "istio.io/istio/pkg/log"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	ctrlzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	apiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	// AutoProvision controls whether the controller will be responsible for provisioning dynamic
	// infrastructure for the Gateway API.
	AutoProvision = true
)

var setupLog = ctrl.Log.WithName("setup")

type StartOptions struct {
	Cache       envoycache.Cache
	KrtDebugger *krt.DebugHandler

	XdsHost string
	XdsPort int32
}

type StartConfig struct {
	Dev        bool
	StartOpts  *StartOptions
	RestConfig *rest.Config
	Client     istiokube.Client

	// todo: extra plugin

	// krt collection
	AugmentedPods krt.Collection[krtcollections.LocalityPod]
	UniqueClients krt.Collection[ir.UniqlyConnectedClient]

	// krt opts
	KrtOptions krtutil.KrtOptions
}

type ControllerBuilder struct {
	// todo: proxy Syncer
	cfg          StartConfig
	mgr          ctrl.Manager
	isOurGateway func(gw *apiv1.Gateway) bool
	settings     settings.Settings
}

func NewControllerBuilder(ctx context.Context, cfg StartConfig) (*ControllerBuilder, error) {
	// setup log
	var opts []ctrlzap.Opts
	loggingOptions := istiolog.DefaultOptions()
	if cfg.Dev {
		setupLog.Info("starting log in dev mode")
		opts = append(opts, ctrlzap.UseDevMode(true))
		loggingOptions.SetDefaultOutputLevel(istiolog.OverrideScopeName, istiolog.DebugLevel)
	}
	ctrl.SetLogger(ctrlzap.New(opts...))
	istiolog.Configure(loggingOptions)

	// setup scheme
	scheme := DefaultScheme()

	// todo: extend scheme

	// setup manager
	mgrOpts := ctrl.Options{
		BaseContext:            func() context.Context { return ctx },
		Scheme:                 scheme,
		PprofBindAddress:       ":9099",
		HealthProbeBindAddress: ":9093",
		Metrics: metricsserver.Options{
			BindAddress: ":9092",
		},
		Controller: config.Controller{
			// disable the name validation here for test
			SkipNameValidation: ptr.To[bool](true),
		},
	}
	mgr, err := ctrl.NewManager(cfg.RestConfig, mgrOpts)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		return nil, err
	}

	mgr.AddHealthzCheck("ping-ready", healthz.Ping)

	// todo: add extentions & proxy syncer

	setupLog.Info("starting controoller builder")
	return &ControllerBuilder{
		cfg: cfg,
		mgr: mgr,
	}, nil
}

// Start starts the controller.
func (c *ControllerBuilder) Start(ctx context.Context) error {
	logger := contextutils.LoggerFrom(ctx).Desugar()
	logger.Info("starting gateway controller")

	xdsHost, xdsPort := c.cfg.StartOpts.XdsHost, c.cfg.StartOpts.XdsPort
	if xdsHost == "" {
		return ctx.Err()
	}

	logger.Info("get xds address for deployer", zap.String("xds_host", xdsHost), zap.Int("xds_port", int(xdsPort)))

	// todo: fix extend plugin & aws info

	gwCfg := GatewayConfig{
		Mgr:            c.mgr,
		OurGateway:     c.isOurGateway,
		ControllerName: wellknown.GatewayControllerName,
		// controller will be responsible for provisioning dynamic infrastructure for the Gateway API.
		AutoProvision: AutoProvision,
		ControlPlane: &deployer.ControlPlaneInfo{
			XdsHost: xdsHost,
			XdsPort: xdsPort,
		},
	}
	if err := NewBaseGatewayController(ctx, gwCfg); err != nil {
		setupLog.Error(err, "unable to create controller")
		return err
	}
	return c.mgr.Start(ctx)
}
