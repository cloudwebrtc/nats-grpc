package rpc

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/cloudwebrtc/nats-grpc/pkg/utils"
	"github.com/golang/protobuf/proto"
	"github.com/nats-io/go-nats"
	"github.com/sirupsen/logrus"
	rpc_status "google.golang.org/genproto/googleapis/rpc/status"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type Client struct {
	nc      NatsConn
	ctx     context.Context
	cancel  context.CancelFunc
	log     *logrus.Entry
	streams map[string]*clientStream
	id      string
	mu      sync.Mutex
}

func NewClient(nc NatsConn, id string) *Client {
	c := &Client{
		nc:      nc,
		id:      id,
		streams: make(map[string]*clientStream),
		log:     logrus.WithField("cli", ""),
	}
	c.ctx, c.cancel = context.WithCancel(context.Background())
	return c
}

// Stop gracefully stops a Client
func (p *Client) Stop() {
	p.cancel()
	for name, st := range p.streams {
		err := st.done()
		if err != nil {
			p.log.Errorf("Unsubscribe [%v] failed %v", name, err)
		}
	}
}

func (c *Client) remove(subj string) {
	c.mu.Lock()
	delete(c.streams, subj)
	c.mu.Unlock()
}

// Invoke performs a unary RPC and returns after the response is received
// into reply.
func (c *Client) Invoke(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
	prefix := "nrpc"
	if len(c.id) > 0 {
		prefix = fmt.Sprintf("nrpc.%v", c.id)
	}
	subj := prefix + strings.ReplaceAll(method, "/", ".")
	payload, err := proto.Marshal(args.(proto.Message))
	msg, err := c.nc.Request(subj, payload, 15*time.Second)
	if err != nil {
		if c.nc.LastError() != nil {
			log.Fatalf("%v for request", c.nc.LastError())
		}
		log.Fatalf("%v for request", err)
		return err
	}

	//TODO: send nrpc.End

	st := &rpc_status.Status{}
	err = proto.Unmarshal(msg.Data, st)
	if err == nil && st.Code != 0 {
		return status.ErrorProto(st)
	}

	err = proto.Unmarshal(msg.Data, reply.(proto.Message))
	if err != nil {
		return err
	}

	return nil
}

//NewStream begins a streaming RPC.
func (c *Client) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	prefix := "nrpc"
	if len(c.id) > 0 {
		prefix = fmt.Sprintf("nrpc.%v", c.id)
	}
	subj := prefix + strings.ReplaceAll(method, "/", ".")
	stream := newClientStream(ctx, c, subj, c.log)
	c.mu.Lock()
	c.streams[stream.reply] = stream
	c.mu.Unlock()
	return stream, nil
}

type clientStream struct {
	header    metadata.MD
	trailer   metadata.MD
	context   context.Context
	log       *logrus.Entry
	client    *Client
	subject   string
	reply     string
	sub       *nats.Subscription
	closeSend bool
}

func newClientStream(c context.Context, client *Client, subj string, log *logrus.Entry) *clientStream {
	cli := &clientStream{
		header:    make(metadata.MD),
		trailer:   make(metadata.MD),
		context:   c,
		client:    client,
		log:       log,
		subject:   subj,
		reply:     utils.NewInBox(),
		closeSend: false,
	}

	cli.sub, _ = client.nc.SubscribeSync(cli.reply)
	return cli
}

func (c *clientStream) Header() (metadata.MD, error) {
	return c.header, nil
}

func (c *clientStream) Trailer() metadata.MD {
	return c.trailer
}

func (c *clientStream) CloseSend() error {
	c.closeSend = true
	return nil
}

func (c *clientStream) Context() context.Context {
	return c.context
}

func (c *clientStream) done() error {
	return c.sub.Unsubscribe()
}

func (c *clientStream) SendMsg(m interface{}) error {
	if c.closeSend {
		return fmt.Errorf("closeSend=true")
	}
	payload, err := proto.Marshal(m.(proto.Message))
	if err != nil {
		c.log.Errorf("clientStream.SendMsg failed: %v", err)
		return err
	}
	return c.client.nc.PublishRequest(c.subject, c.reply, payload)
}

func (c *clientStream) RecvMsg(m interface{}) error {
	msg, err := c.sub.NextMsg(time.Second * 10)
	if err != nil {
		c.client.remove(c.reply)
		c.done()
		return err
	}

	//TODO: save header or trailer recevied nrpc.Begin.header/nprc.End.trailer
	//TODO: returns close when nrpc.End recevied.
	// c.client.remove(c.reply)

	return proto.Unmarshal(msg.Data, m.(proto.Message))
}
