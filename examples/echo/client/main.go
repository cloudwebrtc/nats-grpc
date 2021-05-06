package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/cloudwebrtc/nats-grpc/examples/protos/echo"
	nrpc "github.com/cloudwebrtc/nats-grpc/pkg/rpc"
	"github.com/nats-io/nats.go"
	log "github.com/pion/ion-log"
)

const (
	timestampFormat = time.StampNano // "Jan _2 15:04:05.000"
)

func main() {
	var natsURL = nats.DefaultURL
	if len(os.Args) == 2 {
		natsURL = os.Args[1]
	}

	opts := []nats.Option{nats.Name("nats-grpc echo client")}
	opts = nrpc.SetupConnOptions(opts)

	// Connect to the NATS server.
	nc, err := nats.Connect(natsURL, opts...)
	if err != nil {
		log.Errorf("%v", err)
	}
	defer nc.Close()

	ncli := nrpc.NewClient(nc, "someid")

	cli := echo.NewEchoClient(ncli)

	ctx, cancel := context.WithTimeout(context.Background(), 100000*time.Millisecond)
	defer cancel()

	//Request
	reply, err := cli.SayHello(ctx, &echo.HelloRequest{Msg: "hello"})
	if err != nil {
		log.Infof("SayHello: error %v\n", err)
		return
	}
	log.Infof("SayHello: %s\n", reply.GetMsg())

	//Streaming
	stream, err := cli.Echo(ctx)
	if err != nil {
		log.Errorf("%v", err)
	}

	stream.Send(&echo.EchoRequest{
		Msg: "hello",
	})

	i := 1
	for {
		reply, err := stream.Recv()
		if err != nil {
			log.Errorf("Echo: err %s", err)
			break
		}
		log.Infof("EchoReply: reply.Msg => %s, count => %v", reply.Msg, i)

		i++
		if i <= 100 {
			/*
				//stop loop now, close streaming from client side.
				stream.CloseSend()
				break
			*/
			stream.Send(&echo.EchoRequest{
				Msg: fmt.Sprintf("hello-%v", i),
			})
		}
	}

}
