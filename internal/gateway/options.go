package gateway

import (
	"github.com/Brain-Wave-Ecosystem/gateway/internal/middlewares/logging"
	"github.com/Brain-Wave-Ecosystem/go-common/pkg/grpcx/errors"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/siderolabs/grpc-proxy/proxy"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"
	"time"
)

const (
	maxCallAttempts = 5
)

var connectParams = grpc.ConnectParams{
	Backoff: backoff.Config{
		BaseDelay:  time.Second,
		MaxDelay:   time.Second * 5,
		Jitter:     0.1,
		Multiplier: 1.3,
	},
	MinConnectTimeout: time.Second * 3,
}

func standardDialOptions(_ *zap.Logger) []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithConnectParams(connectParams),
		grpc.WithMaxCallAttempts(maxCallAttempts),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.ForceCodecV2(proxy.Codec())),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
	}
}

func standardServerMuxOptions(logger *zap.Logger) []runtime.ServeMuxOption {
	return []runtime.ServeMuxOption{
		runtime.WithErrorHandler(errors.NewCustomErrorHandler(logger)),
		runtime.WithMiddlewares(logging.NewLoggingMiddleware(logger)),
	}
}

func standardServerOptions(_ *zap.Logger) []grpc.ServerOption {
	return []grpc.ServerOption{}
}
