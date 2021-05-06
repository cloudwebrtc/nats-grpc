package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/cloudwebrtc/nats-grpc/examples/protos/echo"
	"github.com/cloudwebrtc/nats-grpc/pkg/rpc"
	"github.com/nats-io/nats.go"
	log "github.com/pion/ion-log"
)

type echoServer struct {
	echo.UnimplementedEchoServer
}

func (e *echoServer) Echo(stream echo.Echo_EchoServer) error {
	i := int(0)
	for {
		req, err := stream.Recv()
		if err != nil {
			log.Errorf("err: " + err.Error())
			return err
		}
		i++
		log.Infof("Echo: req.Msg => %v, count => %v", req.Msg, i)
		stream.Send(&echo.EchoReply{
			Msg: req.Msg + fmt.Sprintf(" world-%v", i),
		})

		if i >= 100 {
			//stop loop now, close streaming from server side.
			return nil
		}
	}
}

func (e *echoServer) SayHello(ctx context.Context, req *echo.HelloRequest) (*echo.HelloReply, error) {
	fmt.Printf("SayHello: req.Msg => %v\n", req.Msg)
	return &echo.HelloReply{Msg: req.Msg + " world"}, nil
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

	// Keep running until ^C.
	fmt.Println("server is running, ^C quits.")
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	close(c)
}
