package gateway

import (
	"context"
	"fmt"
	"github.com/Brain-Wave-Ecosystem/go-common/pkg/consul"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/hashicorp/consul/api"
	"github.com/siderolabs/grpc-proxy/proxy"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type ServiceOption struct {
	Address      string
	RegisterFunc func(ctx context.Context, mux *runtime.ServeMux, conn *grpc.ClientConn) error
	DialOptions  []grpc.DialOption
}

type Gateway struct {
	ctx          context.Context
	cancel       context.CancelFunc
	consul       *api.Client
	runtimeMux   *runtime.ServeMux
	grpcProxyMux *grpc.Server
	plans        []*consul.Plan
	plansInputs  []chan []*api.ServiceEntry
	plansErrCh   chan error
	grpcConns    map[string]*grpc.ClientConn
	logger       *zap.Logger
}

func NewGateway(consulURL string, serviceOpts []*ServiceOption, logger *zap.Logger) (*Gateway, error) {
	var gt Gateway

	runtimeMux := runtime.NewServeMux(standardServerMuxOptions(logger)...)

	cfg := api.DefaultConfig()
	cfg.Address = consulURL

	client, err := api.NewClient(cfg)
	if err != nil {
		logger.Fatal("error creating client client", zap.Error(err))
		return nil, fmt.Errorf("error creating client client: %w", err)
	}

	gt.runtimeMux = runtimeMux
	gt.ctx, gt.cancel = context.WithCancel(context.Background())
	gt.consul = client
	gt.logger = logger
	gt.grpcConns = make(map[string]*grpc.ClientConn)

	var conn *grpc.ClientConn
	for _, opt := range serviceOpts {
		queue := make(chan []*api.ServiceEntry)

		dialOpts := []grpc.DialOption{grpc.WithResolvers(NewBuilder(queue, logger))}
		dialOpts = append(dialOpts, standardDialOptions(logger)...)
		dialOpts = append(dialOpts, opt.DialOptions...)

		conn, err = grpc.NewClient(fmt.Sprintf(customScheme+":///%s", opt.Address), dialOpts...)

		if err != nil {
			logger.Fatal("error creating grpc client", zap.Error(err))
			return nil, fmt.Errorf("error creating grpc client: %w", err)
		}

		gt.grpcConns[opt.Address] = conn

		if err = opt.RegisterFunc(gt.ctx, runtimeMux, conn); err != nil {
			logger.Fatal("error registering service", zap.String("address", opt.Address), zap.Error(err))
			return nil, fmt.Errorf("error registering service: %w", err)
		}

		logger.Debug("registered service", zap.String("address", opt.Address))

		plan := consul.NewPlan(consulURL, opt.Address, queue)

		gt.plans = append(gt.plans, plan)
		gt.plansInputs = append(gt.plansInputs, queue)
	}

	p := NewProxy(gt.grpcConns, logger)

	grpcServerOpts := []grpc.ServerOption{
		grpc.ForceServerCodecV2(proxy.Codec()),
		grpc.UnknownServiceHandler(proxy.TransparentHandler(p.Director)),
	}

	grpcServerOpts = append(grpcServerOpts, standardServerOptions(logger)...)

	grpcProxy := grpc.NewServer(grpcServerOpts...)

	gt.grpcProxyMux = grpcProxy

	return &gt, err
}

func (gt *Gateway) Runtime() *runtime.ServeMux {
	return gt.runtimeMux
}

func (gt *Gateway) Proxy() *grpc.Server {
	return gt.grpcProxyMux
}

func (gt *Gateway) Start() error {
	var errCh = make(chan error, 10)

	for _, plan := range gt.plans {
		plan.Run(errCh)
	}

	gt.plansErrCh = errCh
	go gt.handleWatchErrors()

	gt.logger.Debug("gateway started")

	return nil
}

func (gt *Gateway) Stop() error {
	for _, plan := range gt.plans {
		plan.Stop()
	}

	var errs error
	var err error

	for _, conn := range gt.grpcConns {
		if err = conn.Close(); err != nil {
			gt.logger.Error("error closing grpc connection", zap.String("target", conn.Target()), zap.Error(err))
			errs = multierr.Append(errs, err)
		}
	}

	gt.cancel()

	gt.logger.Debug("gateway stopped")

	return errs
}

func (gt *Gateway) handleWatchErrors() {
	for err := range gt.plansErrCh {
		if err != nil {
			gt.logger.Warn("plan watch error", zap.Error(err))
		}
	}
}
