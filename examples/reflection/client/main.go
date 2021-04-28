package main

import (
	"context"
	"log"
	"os"
	"time"

	nrpc "github.com/cloudwebrtc/nats-grpc/pkg/rpc"
	"github.com/cloudwebrtc/nats-grpc/pkg/rpc/reflection"
	"github.com/jhump/protoreflect/grpcreflect"
	"github.com/nats-io/nats.go"
	rpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
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
		log.Fatal(err)
	}
	defer nc.Close()

	ncli := nrpc.NewClient(nc, "someid")

	ctx, cancel := context.WithTimeout(context.Background(), 100000*time.Millisecond)
	defer cancel()

	rc := grpcreflect.NewClient(ctx, rpb.NewServerReflectionClient(ncli))

	reflector := reflection.NewReflector(rc)
	list, err := reflector.ListServices()
	if err != nil {
		log.Printf("ListServices: error %v\n", err)
		return
	}

	log.Printf("ListServices: %v\n", list)

	for _, svc := range list {
		log.Printf("Service => %v\n", svc)

		mds, err := reflector.DescribeService(svc)
		if err != nil {
			return
		}

		for _, md := range mds {
			log.Printf("Method => %v\n", md.GetName())
		}
	}
}
