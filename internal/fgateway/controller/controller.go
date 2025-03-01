package controller

import (
	"context"
	"fmt"

	"github.com/fleezesd/fgateway/internal/fgateway/deployer"
	"github.com/fleezesd/fgateway/internal/fgateway/wellknown"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	apiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	// field name used for indexing
	GatewayParamsField = "gateway-params"
)

type GatewayConfig struct {
	Mgr manager.Manager

	OurGateway             func(gw *apiv1.Gateway) bool
	ControllerName         string
	Dev                    bool
	AutoProvision          bool
	EnableIstioIntegration bool
	ControlPlane           *deployer.ControlPlaneInfo
	Aws                    *deployer.AwsInfo
}

type controllerBuilder struct {
	cfg        GatewayConfig
	reconciler *controllerReconciler
}

func NewBaseGatewayController(ctx context.Context, cfg GatewayConfig) error {
	log := log.FromContext(ctx)
	log.V(5).Info("starting controller", "controllerName", cfg.ControllerName)

	controllerBuilder := &controllerBuilder{
		cfg: cfg,
		reconciler: &controllerReconciler{
			cli:    cfg.Mgr.GetClient(),
			scheme: cfg.Mgr.GetScheme(),
		},
	}
	return run(ctx,
		controllerBuilder.watchGatewayClass,
		controllerBuilder.addGatewayParamsIndex,
	)
}

// run executes a series of controllerBuilder watch functions sequentially with the given context
func run(ctx context.Context, funcs ...func(ctx context.Context) error) error {
	for _, fn := range funcs {
		if err := fn(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (c *controllerBuilder) watchGatewayClass(ctx context.Context) error {
	// make controller manager and with event filter
	return ctrl.NewControllerManagedBy(c.cfg.Mgr).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		WithEventFilter(predicate.NewPredicateFuncs(func(object client.Object) bool {
			if gatewayClass, ok := object.(*apiv1.GatewayClass); ok {
				return gatewayClass.Spec.ControllerName == apiv1.GatewayController(c.cfg.ControllerName)
			}
			return false
		})).
		For(&apiv1.GatewayClass{}).
		Complete(reconcile.Func(c.reconciler.ReconcileGatewayClass))
}

func (c *controllerBuilder) watchGateway(ctx context.Context) error {
	// todo: add watch gateway logic
	log := log.FromContext(ctx)

	log.Info("creating deployer",
		"controller name", c.cfg.ControllerName,
		"server", c.cfg.ControlPlane.XdsHost,
		"port", c.cfg.ControlPlane.XdsPort,
	)
	deployer, err := deployer.NewDeployer(c.cfg.Mgr.GetClient(), &deployer.Inputs{
		ControllerName:          c.cfg.ControllerName,
		Dev:                     c.cfg.Dev,
		IstioIntegrationEnabled: c.cfg.EnableIstioIntegration,
		ControlPlane:            c.cfg.ControlPlane,
		Aws:                     c.cfg.Aws,
	})
	if err != nil {
		return err
	}

	gvks, err := deployer.GetGvksToWatch(ctx)
	if err != nil {
		return err
	}

	buildr := ctrl.NewControllerManagedBy(c.cfg.Mgr).
		For(&apiv1.Gateway{}, builder.WithPredicates(predicate.NewPredicateFuncs(func(object client.Object) bool {
			// We only care about Gateways that use our GatewayClass
			if gw, ok := object.(*apiv1.Gateway); ok {
				return c.cfg.OurGateway(gw)
			}
			return false
		}),
			predicate.Or(
				predicate.AnnotationChangedPredicate{},
				predicate.GenerationChangedPredicate{},
			),
		))
	_ = c.cfg.Mgr.GetClient()

	// todo: watch for changes in gatewayparameters
	for _, gvk := range gvks {
		obj, err := c.cfg.Mgr.GetScheme().New(gvk)
		if err != nil {
			return err
		}
		clientObj, ok := obj.(client.Object)
		if !ok {
			return fmt.Errorf("object %T is not a client.Object", obj)
		}
		log.Info("watching gvk as gateway child", "gvk", gvk)
		// unless it's a service, we don't care about the status
		var opts []builder.OwnsOption
		if shouldIgnoreStatusChild(gvk) {
			opts = append(opts, builder.WithPredicates(predicate.GenerationChangedPredicate{}))
		}
		buildr.Owns(clientObj, opts...)
	}
	return nil
}

func shouldIgnoreStatusChild(gvk schema.GroupVersionKind) bool {
	// avoid triggering on pod changes that update deployment status
	return gvk.Kind == "Deployment"
}

func (c *controllerBuilder) addGatewayParamsIndex(ctx context.Context) error {
	// fix gateway mgr indexer
	return c.cfg.Mgr.GetFieldIndexer().IndexField(ctx, &apiv1.Gateway{}, GatewayParamsField, gatewayToParams)
}

func gatewayToParams[T client.IndexerFunc](obj client.Object) []string {
	gw, ok := obj.(*apiv1.Gateway)
	if !ok {
		panic(fmt.Sprintf("wrong type %T provided to indexer, expected Gateway", obj))
	}
	gwpName := gw.GetAnnotations()[wellknown.GatewayParametersAnnonationName]
	if gwpName != "" {
		return []string{gwpName}
	}
	return []string{}
}
