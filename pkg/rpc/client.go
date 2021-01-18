package rpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"

	"github.com/cloudwebrtc/nats-grpc/pkg/protos/nrpc"
	"github.com/cloudwebrtc/nats-grpc/pkg/utils"
	"github.com/golang/protobuf/proto"
	"github.com/nats-io/go-nats"
	"github.com/sirupsen/logrus"
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

// Invoke performs a unary RPC and returns after the request is received
// into reply.
func (c *Client) Invoke(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
	prefix := "nrpc"
	if len(c.id) > 0 {
		prefix = fmt.Sprintf("nrpc.%v", c.id)
	}
	subj := prefix + strings.ReplaceAll(method, "/", ".")
	stream := newClientStream(ctx, c, subj, c.log, opts...)
	c.mu.Lock()
	c.streams[stream.reply] = stream
	c.mu.Unlock()
	return stream.Invoke(ctx, method, args, reply, opts...)
}

//NewStream begins a streaming RPC.
func (c *Client) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	prefix := "nrpc"
	if len(c.id) > 0 {
		prefix = fmt.Sprintf("nrpc.%v", c.id)
	}
	subj := prefix + strings.ReplaceAll(method, "/", ".")
	stream := newClientStream(ctx, c, subj, c.log, opts...)
	c.mu.Lock()
	c.streams[stream.reply] = stream
	c.mu.Unlock()
	return stream, nil
}

type clientStream struct {
	md        *metadata.MD
	header    *metadata.MD
	trailer   *metadata.MD
	ctx       context.Context
	cancel    context.CancelFunc
	log       *logrus.Entry
	client    *Client
	subject   string
	reply     string
	msgCh     chan *nats.Msg
	sub       *nats.Subscription
	closed    bool
	recvRead  <-chan []byte
	recvWrite chan<- []byte
	hasBegun  bool
}

func newClientStream(ctx context.Context, client *Client, subj string, log *logrus.Entry, opts ...grpc.CallOption) *clientStream {
	stream := &clientStream{
		client:  client,
		log:     log,
		subject: subj,
		reply:   utils.NewInBox(),
		closed:  false,
	}
	stream.ctx, stream.cancel = context.WithCancel(ctx)

	recv := make(chan []byte, 1)
	stream.recvRead = recv
	stream.recvWrite = recv

	stream.msgCh = make(chan *nats.Msg, 1)
	stream.sub, _ = client.nc.ChanSubscribe(stream.reply, stream.msgCh)

	md, ok := metadata.FromOutgoingContext(ctx)
	if ok {
		//log.Printf("stream outgoing md => %v", md)
		stream.md = &md
	}

	for _, o := range opts {
		switch o := o.(type) {
		case grpc.HeaderCallOption:
			//log.Printf("o.HeaderAddr => %v", o.HeaderAddr)
			stream.header = o.HeaderAddr
		case grpc.TrailerCallOption:
			//log.Printf("o.TrailerAddr => %v", o.TrailerAddr)
			stream.trailer = o.TrailerAddr
		case grpc.PeerCallOption:
		case grpc.PerRPCCredsCallOption:
		case grpc.FailFastCallOption:
		case grpc.MaxRecvMsgSizeCallOption:
		case grpc.MaxSendMsgSizeCallOption:
		case grpc.CompressorCallOption:
		case grpc.ContentSubtypeCallOption:
		}
	}

	go stream.ReadMsg()
	return stream
}

func (c *clientStream) Header() (metadata.MD, error) {
	return *c.header, nil
}

func (c *clientStream) Trailer() metadata.MD {
	return *c.trailer
}

func (c *clientStream) CloseSend() error {
	c.log.Info("Client CloseSend")
	c.writeEnd(&nrpc.End{
		Status: status.Convert(nil).Proto(),
	})
	return c.done()
}

func (c *clientStream) close(err error) {
	c.writeEnd(&nrpc.End{
		Status: status.Convert(err).Proto(),
	})
	c.done()
}

func (c *clientStream) Context() context.Context {
	return c.ctx
}

func (c *clientStream) onMessage(msg *nats.Msg) error {
	response := &nrpc.Response{}
	err := proto.Unmarshal(msg.Data, response)
	if err != nil {
		c.log.WithField("data", string(msg.Data)).Error("unknown message")
		return err
	}

	switch r := response.Type.(type) {
	case *nrpc.Response_Begin:
		//c.log.WithField("call", r.Begin).Info("recv call")
		c.processBegin(r.Begin)
	case *nrpc.Response_Data:
		//c.log.WithField("data", r.Data).Info("recv data")
		c.processData(r.Data)
	case *nrpc.Response_End:
		//c.log.WithField("end", r.End).Info("recv end")
		c.processEnd(r.End)
		c.close(nil)
		//request ended.
		return fmt.Errorf("Ended")
	}
	return nil
}

func (c *clientStream) ReadMsg() error {
	for {
		select {
		case <-c.ctx.Done():
			return c.ctx.Err()
		case msg, ok := <-c.msgCh:
			if ok {
				err := c.onMessage(msg)
				if err != nil {
					return err
				}
				break
			}
			return io.EOF
		}
	}
}

func (c *clientStream) done() error {
	if !c.closed {
		c.closed = true
		c.cancel()
		close(c.msgCh)
		c.client.remove(c.subject)
		return c.sub.Unsubscribe()
	}
	return errors.New("Client Streaming already closed")
}

func (c *clientStream) SendMsg(m interface{}) error {
	if c.closed {
		return fmt.Errorf("client streaming closed=true")
	}

	if !c.hasBegun {
		c.hasBegun = true

		md := utils.MakeMetadata(*c.md)
		call := &nrpc.Call{
			Method:   c.subject,
			Metadata: md,
		}
		//write call with metatdata
		c.writeCall(call)
	}

	payload, err := proto.Marshal(m.(proto.Message))
	if err != nil {
		c.log.Errorf("clientStream.SendMsg failed: %v", err)
		return err
	}
	data := &nrpc.Data{
		Data: payload,
	}
	//write grpc args
	return c.writeData(data)
}

func (c *clientStream) RecvMsg(m interface{}) error {
	select {
	case <-c.ctx.Done():
		return c.ctx.Err()
	case bytes, ok := <-c.recvRead:
		if ok {
			return proto.Unmarshal(bytes, m.(proto.Message))
		}
		return io.EOF
	}
}

func (c *clientStream) Invoke(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
	payload, err := proto.Marshal(args.(proto.Message))
	if err != nil {
		log.Fatalf("%v for request", err)
		return err
	}

	//write call with metatdata
	c.writeCall(&nrpc.Call{
		Method:   method,
		Metadata: utils.MakeMetadata(*c.md),
	})

	//write grpc args
	c.writeData(&nrpc.Data{
		Data: payload,
	})

	err = c.RecvMsg(reply)

	c.CloseSend()

	return err
}

func (c *clientStream) writeRequest(request *nrpc.Request) error {
	//c.log.WithField("request", request).Info("send")
	data, err := proto.Marshal(request)
	if err != nil {
		return err
	}
	return c.client.nc.PublishRequest(c.subject, c.reply, data)
}

func (c *clientStream) writeCall(call *nrpc.Call) error {
	return c.writeRequest(&nrpc.Request{
		Type: &nrpc.Request_Call{
			Call: call,
		},
	})
}

func (c *clientStream) writeData(data *nrpc.Data) error {
	return c.writeRequest(&nrpc.Request{
		Type: &nrpc.Request_Data{
			Data: data,
		},
	})
}

func (c *clientStream) writeEnd(end *nrpc.End) error {
	return c.writeRequest(&nrpc.Request{
		Type: &nrpc.Request_End{
			End: end,
		},
	})
}

func (c *clientStream) processBegin(begin *nrpc.Begin) error {
	c.log = c.log.WithField("nrpc.Begin", begin.Header)
	if begin.Header != nil && c.header != nil {
		if *c.header == nil {
			*c.header = metadata.MD{}
		}
		for hdr, data := range begin.Header.Md {
			c.header.Append(hdr, data.Values...)
		}
	}
	return nil
}

func (c *clientStream) processData(data *nrpc.Data) {
	if c.recvWrite == nil {
		c.log.Error("data received after client closeSend")
		return
	}
	c.recvWrite <- data.Data
}

func (c *clientStream) processEnd(end *nrpc.End) {

	if end.Trailer != nil && c.trailer != nil {
		if *c.trailer == nil {
			*c.trailer = metadata.MD{}
		}
		for hdr, data := range end.Trailer.Md {
			c.trailer.Append(hdr, data.Values...)
		}
	}

	if end.Status != nil {
		c.log.WithField("status", end.Status).Info("cancel")
	} else {
		c.log.Info("Server CloseSend")
	}
}
