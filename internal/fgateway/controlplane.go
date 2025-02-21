package fgateway

import (
	"context"
	"net"

	envoy_service_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/service/cluster/v3"
	envoy_service_discovery_v3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	envoy_service_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/service/endpoint/v3"
	envoy_service_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/service/listener/v3"
	envoy_service_route_v3 "github.com/envoyproxy/go-control-plane/envoy/service/route/v3"
	envoycache "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	xdsserver "github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"github.com/fleezesd/fgateway/pkg/xds"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"github.com/solo-io/go-utils/contextutils"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func NewControlPlane(
	ctx context.Context,
	bindAddr net.Addr,
	callbacks xdsserver.Callbacks,
) (envoycache.SnapshotCache, error) {
	lis, err := net.Listen(bindAddr.Network(), bindAddr.String())
	if err != nil {
		return nil, err
	}
	return NewControlPlaneWithListener(ctx, lis, callbacks)
}

func NewControlPlaneWithListener(
	ctx context.Context,
	lis net.Listener,
	callbacks xdsserver.Callbacks,
) (envoycache.SnapshotCache, error) {
	logger := contextutils.LoggerFrom(ctx).Desugar()
	serverOpts := []grpc.ServerOption{
		grpc.StreamInterceptor(
			grpc_middleware.ChainStreamServer(
				// grpc zap log
				grpc_zap.StreamServerInterceptor(zap.NewNop()),
				func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
					logger.Debug("gRPC call", zap.String("method", info.FullMethod))
					return handler(srv, ss)
				},
			),
		),
	}
	grpcServer := grpc.NewServer(serverOpts...)

	// snapshotCache maintains a single versioned snapshot of responses per node
	snapshotCache := envoycache.NewSnapshotCache(true, xds.NewNodeRoleHasher(), logger.Sugar()) // ads(Aggregated Discovery Service)

	xdsServer := xdsserver.NewServer(ctx, snapshotCache, callbacks)
	reflection.Register(grpcServer) // reflection register for grpc

	// register xds server
	envoy_service_endpoint_v3.RegisterEndpointDiscoveryServiceServer(grpcServer, xdsServer)    // EDS enpoint discovery
	envoy_service_cluster_v3.RegisterClusterDiscoveryServiceServer(grpcServer, xdsServer)      // CDS cluster discovery
	envoy_service_route_v3.RegisterRouteDiscoveryServiceServer(grpcServer, xdsServer)          // RDS route discovery
	envoy_service_listener_v3.RegisterListenerDiscoveryServiceServer(grpcServer, xdsServer)    // LDS listener discovery
	envoy_service_discovery_v3.RegisterAggregatedDiscoveryServiceServer(grpcServer, xdsServer) // ADS aggregated discovery

	go grpcServer.Serve(lis)
	go func() {
		<-ctx.Done()
		grpcServer.GracefulStop()
	}()

	return snapshotCache, nil
}
