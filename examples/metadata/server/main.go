package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/cloudwebrtc/nats-grpc/examples/protos/echo"
	"github.com/cloudwebrtc/nats-grpc/pkg/rpc"
	"github.com/nats-io/nats.go"
	log "github.com/pion/ion-log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type echoServer struct {
	echo.UnimplementedEchoServer
}

func (e *echoServer) SayHello(ctx context.Context, req *echo.HelloRequest) (*echo.HelloReply, error) {
	method, _ := grpc.Method(ctx)
	log.Infof("method: %v", method)

	// Read metadata from client.
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.DataLoss, "SayHello: failed to get metadata")
	}
	log.Infof("md: %v", md)
	if t, ok := md["timestamp"]; ok {
		log.Infof("timestamp from metadata:")
		for i, e := range t {
			log.Infof(" %d. %s\n", i, e)
		}
	}

	// Create and send header.
	header := metadata.New(map[string]string{"location": "MTV", "timestamp": time.Now().Format(time.StampNano)})
	grpc.SetHeader(ctx, header)
	grpc.SetHeader(ctx, metadata.Pairs("Pre-Response-Metadata", "Is-sent-as-headers-unary"))
	grpc.SetTrailer(ctx, metadata.Pairs("Post-Response-Metadata", "Is-sent-as-trailers-unary"))
	return &echo.HelloReply{Msg: req.Msg + " world"}, nil //status.Errorf(codes.Unimplemented, "method SayHello not implemented")
}

func main() {

	var natsURL = nats.DefaultURL
	if len(os.Args) == 2 {
		natsURL = os.Args[1]
	}

	opts := []nats.Option{nats.Name("nats-grpc echo client")}
	opts = rpc.SetupConnOptions(opts)

	// Connect to the NATS server.
	nc, err := nats.Connect(natsURL, opts...)
	if err != nil {
		log.Errorf("%v", err)
	}
	defer nc.Close()

	ncs := rpc.NewServer(nc, "someid")
	echo.RegisterEchoServer(ncs, &echoServer{})

	// Keep running until ^C.
	fmt.Println("server is running, ^C quits.")
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	close(c)
}
