package server

import (
	"context"
	"errors"
	"fmt"
	auth "github.com/Brain-Wave-Ecosystem/gateway/gen/auth"
	courses "github.com/Brain-Wave-Ecosystem/gateway/gen/courses"
	users "github.com/Brain-Wave-Ecosystem/gateway/gen/users"
	"github.com/Brain-Wave-Ecosystem/gateway/internal/config"
	"github.com/Brain-Wave-Ecosystem/gateway/internal/gateway"
	"github.com/Brain-Wave-Ecosystem/go-common/pkg/abstractions"
	"github.com/Brain-Wave-Ecosystem/go-common/pkg/log"
	"github.com/DavidMovas/gopherbox/pkg/closer"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"net"
	"net/http"
)

var _ abstractions.Server = (*Server)(nil)

type Server struct {
	gateway *gateway.Gateway
	logger  *log.Logger
	cfg     *config.Config
	closer  *closer.Closer
}

func NewServer(_ context.Context, cfg *config.Config) (*Server, error) {
	cl := closer.NewCloser()

	logger, err := log.NewLogger(cfg.Local, cfg.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("error initializing logger: %w", err)
	}

	cl.Push(logger.Stop)

	srvOpts := []*gateway.ServiceOption{
		{
			Address:      "users_service",
			RegisterFunc: users.RegisterUsersServiceHandler,
		},
		{
			Address:      "auth_service",
			RegisterFunc: auth.RegisterAuthServiceHandler,
		},
		{
			Address:      "courses_service",
			RegisterFunc: courses.RegisterCoursesServiceHandler,
		},
	}

	gt, err := gateway.NewGateway(cfg.ConsulURL, srvOpts, logger.Zap())
	if err != nil {
		logger.Zap().Error("error initializing gateway", zap.Error(err))
		return nil, err
	}

	return &Server{
		gateway: gt,
		logger:  logger,
		cfg:     cfg,
		closer:  cl,
	}, nil
}

func (s *Server) Start() error {
	httpPort := s.cfg.HTTPPort
	grpcPort := s.cfg.GRPCPort
	logger := s.logger.Zap()

	if err := s.gateway.Start(); err != nil {
		logger.Error("error starting gateway", zap.Error(err))
		return err
	}

	group := errgroup.Group{}

	group.Go(func() error {
		logger.Info("starting http runtime server", zap.String("port", httpPort))

		if ls, err := net.Listen("tcp", fmt.Sprintf(":%s", httpPort)); err == nil {
			runtime := s.gateway.Runtime()
			tcpSrv := &http.Server{Handler: runtime}

			s.closer.PushIO(ls)
			s.closer.PushIO(tcpSrv)

			if err = tcpSrv.Serve(ls); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Error("error serving http runtime server", zap.Error(err))
				return err
			}

			logger.Info("http runtime server stopped")

			return nil
		} else {
			logger.Error("error starting http runtime server", zap.Error(err))
			return err
		}
	})

	group.Go(func() error {
		logger.Info("starting grpc proxy server", zap.String("port", grpcPort))

		if ls, err := net.Listen("tcp", fmt.Sprintf(":%s", grpcPort)); err == nil {
			proxy := s.gateway.Proxy()

			s.closer.PushIO(ls)
			s.closer.PushNE(proxy.GracefulStop)

			if err = proxy.Serve(ls); err != nil {
				logger.Error("error serving grpc proxy server", zap.Error(err))
				return err
			}

			logger.Info("grpc proxy server stopped")

			return nil
		} else {
			logger.Error("error starting grpc proxy server", zap.Error(err))
			return err
		}
	})

	s.closer.Push(s.gateway.Stop)

	return group.Wait()
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Zap().Info("Shutting down server...")
	return s.closer.Close(ctx)
}
