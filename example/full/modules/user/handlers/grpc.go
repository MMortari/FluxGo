package handlers

import (
	"context"

	fluxgo "github.com/MMortari/FluxGo"
	userpb "github.com/MMortari/FluxGo/example/full/shared/pb/user"
	"github.com/MMortari/FluxGo/example/full/shared/repositories"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// HandlerUserGrpc implements UserServiceServer (proto-generated) and
// fluxgo.GrpcHandlerInterface. Register it with fluxgo.GrpcDef[HandlerUserGrpc].
type HandlerUserGrpc struct {
	userpb.UnimplementedUserServiceServer
	repository *repositories.UserRepository
	logger     *fluxgo.Logger
}

func HandlerUserGrpcStart(repository *repositories.UserRepository, logger *fluxgo.Logger) *HandlerUserGrpc {
	return &HandlerUserGrpc{repository: repository, logger: logger}
}

func (h *HandlerUserGrpc) GetUser(ctx context.Context, req *userpb.GetUserRequest) (*userpb.GetUserResponse, error) {
	log := h.logger.CreateLogger(ctx)
	log.Infof("GetUser gRPC called: id_user=%s", req.GetIdUser())

	user, err := h.repository.GetUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "error getting user")
	}
	if user == nil {
		return nil, status.Errorf(codes.NotFound, "user not found")
	}

	return &userpb.GetUserResponse{
		User: &userpb.User{
			Id:   user.ID,
			Name: user.Name,
		},
	}, nil
}

// RegisterGrpc implements fluxgo.GrpcHandlerInterface.
func (h *HandlerUserGrpc) RegisterGrpc(server *grpc.Server) {
	userpb.RegisterUserServiceServer(server, h)
}
