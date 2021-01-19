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
}
