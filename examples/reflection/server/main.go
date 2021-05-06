package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/cloudwebrtc/nats-grpc/examples/protos/echo"
	"github.com/cloudwebrtc/nats-grpc/pkg/rpc"
	"github.com/cloudwebrtc/nats-grpc/pkg/rpc/reflection"
	"github.com/nats-io/nats.go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	log "github.com/pion/ion-log"
)

type echoServer struct {
	echo.UnimplementedEchoServer
}

func (e *echoServer) Echo(stream echo.Echo_EchoServer) error {
	return status.Errorf(codes.Unimplemented, "method Echo not implemented")
}

func (e *echoServer) SayHello(ctx context.Context, req *echo.HelloRequest) (*echo.HelloReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SayHello not implemented")
}

func main() {

	var natsURL = nats.DefaultURL
	if len(os.Args) == 2 {
		natsURL = os.Args[1]
	}
	opts := []nats.Option{nats.Name("nats-grpc echo service")}
	opts = rpc.SetupConnOptions(opts)
	// Connect to the NATS server.
	nc, err := nats.Connect(natsURL, opts...)
	if err != nil {
		log.Errorf("%v", err)
	}
	defer nc.Close()

	ncs := rpc.NewServer(nc, "someid")
	echo.RegisterEchoServer(ncs, &echoServer{})

	// Register reflection service on gRPC server.
	reflection.Register(ncs)

	// Keep running until ^C.
	fmt.Println("server is running, ^C quits.")
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	close(c)
}
