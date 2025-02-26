package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type controllerReconciler struct {
	cli    client.Client
	scheme *runtime.Scheme
}

func (r *controllerReconciler) ReconcileGatewayClass(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}
