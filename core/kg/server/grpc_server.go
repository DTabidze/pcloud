package server

import (
	"fmt"
	"net"
	"os"

	"github.com/giolekva/pcloud/core/kg/log"
	"github.com/giolekva/pcloud/core/kg/model"
	"github.com/giolekva/pcloud/core/kg/store"
	"google.golang.org/grpc"
)

// GRPCServerImpl grpc server implementation
type GRPCServerImpl struct {
	Log    *log.Logger
	srv    *grpc.Server
	config *model.Config
	store  store.Store
}

var _ Server = &GRPCServerImpl{}

// NewGRPCServer creates new GRPC Server
func NewGRPCServer(logger *log.Logger, config *model.Config, store store.Store) Server {
	a := &GRPCServerImpl{
		Log:    logger,
		config: config,
		store:  store,
	}

	pwd, _ := os.Getwd()
	a.Log.Info("GRPC server current working", log.String("directory", pwd))
	return a
}

// Start method starts a grpc server
func (a *GRPCServerImpl) Start() error {
	a.Log.Info("Starting GRPC Server...")

	// settings := model.NewConfig().SqlSettings
	// a.store = sqlstore.New(settings)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", a.config.GRPCSettings.Port))
	if err != nil {
		a.Log.Error("failed to listen: %v", log.Err(err))
		return err
	}

	a.srv = grpc.NewServer()

	a.Log.Info("GRPC Server is listening on", log.Int("port", a.config.GRPCSettings.Port))
	if err := a.srv.Serve(lis); err != nil {
		a.Log.Error("failed to serve rpc: %v", log.Err(err))
		return err
	}
	return nil
}

// Shutdown method shuts grpc server down
func (a *GRPCServerImpl) Shutdown() error {
	a.Log.Info("Stopping GRPC Server...")
	a.srv.GracefulStop()
	a.Log.Info("GRPC Server stopped")
	return nil
}
