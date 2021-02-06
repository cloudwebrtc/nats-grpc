package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/cloudwebrtc/nats-grpc/examples/protos/echo"
	"github.com/cloudwebrtc/nats-grpc/pkg/rpc"
	nrpc "github.com/cloudwebrtc/nats-grpc/pkg/rpc"
	"github.com/nats-io/nats.go"
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
	opts = rpc.SetupConnOptions(opts)

	// Connect to the NATS server.
	nc, err := nats.Connect(natsURL, opts...)
	if err != nil {
		log.Fatal(err)
	}
	defer nc.Close()

	ncli := nrpc.NewClient(nc, "someid")

	cli := echo.NewEchoClient(ncli)

	ctx, cancel := context.WithTimeout(context.Background(), 1000*time.Millisecond)
	defer cancel()

	//Request
	reply, err := cli.SayHello(ctx, &echo.HelloRequest{Msg: "hello"})
	if err != nil {
		log.Printf("SayHello: error %v\n", err)
		return
	}
	log.Printf("SayHello: %s\n", reply.GetMsg())

	//Streaming
	stream, err := cli.Echo(ctx)
	if err != nil {
		log.Fatal(err)
	}

	stream.Send(&echo.EchoRequest{
		Msg: "hello",
	})

	i := 1
	for {
		reply, err := stream.Recv()
		if err != nil {
			log.Fatalf("Echo: err %s", err)
			break
		}
		log.Printf("EchoReply: reply.Msg => %s, count => %v", reply.Msg, i)

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
