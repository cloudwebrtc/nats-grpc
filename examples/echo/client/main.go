package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/cloudwebrtc/nats-grpc/pkg/protos/echo"
	nrpc "github.com/cloudwebrtc/nats-grpc/pkg/rpc"
	"github.com/nats-io/go-nats"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

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

	ncli := nrpc.NewClient(nc, "someid")

	cli := echo.NewEchoClient(ncli)
	/*
		reply, err := cli.SayHello(context.TODO(), &echo.HelloRequest{Msg: "hello"})
		if err != nil {
			log.Printf("SayHello: error %v\n", err)
			return
		}
		log.Printf("SayHello: %s\n", reply.GetMsg())
	*/
	header := metadata.New(map[string]string{"key1": "val1", "key2": "val2"})
	trailer := metadata.New(map[string]string{"key3": "val3", "key4": "val4"})

	stream, err := cli.Echo(context.TODO(),
		grpc.Header(&header), // will retrieve header
		grpc.Trailer(&trailer))
	if err != nil {
		log.Fatal(err)
	}

	stream.Send(&echo.EchoRequest{
		Msg: "hello",
	})
	for {
		reply, err := stream.Recv()
		if err != nil {
			log.Fatalf("Echo: err %s", reply.GetMsg())
			break
		}
		log.Printf("Echo: %s", reply.GetMsg())
		stream.Send(&echo.EchoRequest{
			Msg: "hello",
		})
		//stream.CloseSend()
		//break
	}
}
