package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/cloudwebrtc/nats-grpc/pkg/protos/echo"
	"github.com/cloudwebrtc/nats-grpc/pkg/rpc"
	"github.com/nats-io/go-nats"
)

type echoServer struct {
	echo.UnimplementedEchoServer
}

func (e *echoServer) Echo(stream echo.Echo_EchoServer) error {
	for {
		req, err := stream.Recv()
		if err != nil {
			return err
		}
		stream.Send(&echo.EchoReply{
			Msg: req.Msg + " world",
		})
	}
}

func (e *echoServer) SayHello(ctx context.Context, req *echo.HelloRequest) (*echo.HelloReply, error) {
	return &echo.HelloReply{Msg: req.Msg + " world"}, nil //status.Errorf(codes.Unimplemented, "method SayHello not implemented")
}

func main() {

	var natsURL = nats.DefaultURL
	if len(os.Args) == 2 {
		natsURL = os.Args[1]
	}
	// Connect to the NATS server.
	nc, err := nats.Connect(natsURL, nats.Timeout(5*time.Second))
	if err != nil {
		log.Fatal(err)
	}
	defer nc.Close()

	ncs := rpc.NewServer(nc, "")
	echo.RegisterEchoServer(ncs, &echoServer{})

	// Keep running until ^C.
	fmt.Println("server is running, ^C quits.")
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	close(c)
}
