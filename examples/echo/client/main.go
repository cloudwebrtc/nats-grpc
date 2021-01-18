package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/cloudwebrtc/nats-grpc/pkg/protos/echo"
	nrpc "github.com/cloudwebrtc/nats-grpc/pkg/rpc"
	"github.com/nats-io/go-nats"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	timestampFormat = time.StampNano // "Jan _2 15:04:05.000"
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

	// Create metadata and context.
	md := metadata.Pairs("timestamp", time.Now().Format(timestampFormat))
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	// Make RPC using the context with the metadata.
	var header, trailer metadata.MD
	//Request
	reply, err := cli.SayHello(ctx, &echo.HelloRequest{Msg: "hello"}, grpc.Header(&header), grpc.Trailer(&trailer))
	if err != nil {
		log.Printf("SayHello: error %v\n", err)
		return
	}
	log.Printf("SayHello: %s\n", reply.GetMsg())

	if t, ok := header["timestamp"]; ok {
		fmt.Printf("timestamp from header:\n")
		for i, e := range t {
			fmt.Printf(" %d. %s\n", i, e)
		}
	} else {
		log.Fatal("timestamp expected but doesn't exist in header")
	}
	if l, ok := header["location"]; ok {
		fmt.Printf("location from header:\n")
		for i, e := range l {
			fmt.Printf(" %d. %s\n", i, e)
		}
	} else {
		log.Fatal("location expected but doesn't exist in header")
	}

	//Streaming
	//header := metadata.New(map[string]string{"key1": "val1", "key2": "val2"})
	//trailer := metadata.New(map[string]string{"key3": "val3", "key4": "val4"})

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
			log.Fatalf("Echo: err %s", reply.GetMsg())
			break
		}
		log.Printf("Echo: %s, cnt %v", reply.GetMsg(), i)
		stream.Send(&echo.EchoRequest{
			Msg: "hello",
		})
		i++
		if i >= 100 {
			stream.CloseSend()
			break
		}
		//stream.CloseSend()
		//break
	}
}
