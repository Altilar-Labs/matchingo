package server

import (
	"github.com/erain9/matchingo/pkg/api/proto"
	"google.golang.org/grpc"
)

// RegisterOrderBookService registers the order book service with the provided gRPC server
func RegisterOrderBookService(grpcServer *grpc.Server, service *GRPCOrderBookService) {
	proto.RegisterOrderBookServiceServer(grpcServer, service)
}
