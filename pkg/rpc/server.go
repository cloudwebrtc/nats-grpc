package rpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/nats-io/go-nats"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// redefine grpc.serverMethodHandler as it is not exposed
type serverMethodHandler func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error)

func serverUnaryHandler(srv interface{}, handler serverMethodHandler) handlerFunc {
	return func(s *serverStream) {
		var interceptor grpc.UnaryServerInterceptor = nil
		response, err := handler(srv, s.ctx, s.RecvMsg, interceptor)
		if s.ctx.Err() == nil {
			if err != nil {
				st, _ := status.FromError(err)
				if s.SendMsg(st.Proto()) == nil {
					s.close(err)
				}
				return
			}
			if s.SendMsg(response) == nil {
				s.close(err)
			}
		}
	}
}

func serverStreamHandler(srv interface{}, handler grpc.StreamHandler) handlerFunc {
	return func(s *serverStream) {
		err := handler(srv, s)
		if s.ctx.Err() == nil {
			s.close(err)
		}
	}
}

type handlerFunc func(s *serverStream)

// Server is the interface to gRPC over NATS
type Server struct {
	nc       NatsConn
	ctx      context.Context
	cancel   context.CancelFunc
	log      *logrus.Entry
	handlers map[string]handlerFunc
	streams  map[string]*serverStream
	mu       sync.Mutex
	subs     map[string]*nats.Subscription
	id       string
}

// NewServer creates a new Proxy
func NewServer(nc NatsConn, id string) *Server {
	p := &Server{
		nc:       nc,
		handlers: make(map[string]handlerFunc),
		streams:  make(map[string]*serverStream),
		subs:     make(map[string]*nats.Subscription),
		log:      logrus.WithField("sid", id),
		id:       id,
	}
	p.ctx, p.cancel = context.WithCancel(context.Background())
	return p
}

// Stop gracefully stops a Proxy
func (p *Server) Stop() {
	p.cancel()
	for name, sub := range p.subs {
		err := sub.Unsubscribe()
		if err != nil {
			p.log.Errorf("Unsubscribe [%v] failed %v", name, err)
		}
	}
}

// RegisterService is used to register gRPC services
func (p *Server) RegisterService(sd *grpc.ServiceDesc, ss interface{}) {
	prefix := fmt.Sprintf("nrpc.%v", sd.ServiceName)
	if len(p.id) > 0 {
		prefix = fmt.Sprintf("nrpc.%v.%v", p.id, sd.ServiceName)
	}
	subject := prefix + ".>"
	p.log.Infof("QueueSubscribe: subject => %v, queue => %v", subject, sd.ServiceName)
	sub, _ := p.nc.QueueSubscribe(subject, sd.ServiceName, p.onMessage)

	p.subs[sd.ServiceName] = sub
	for _, it := range sd.Methods {
		desc := it
		path := fmt.Sprintf("%v.%v", prefix, desc.MethodName)
		p.handlers[path] = serverUnaryHandler(ss, serverMethodHandler(desc.Handler))
		p.log.Infof("RegisterService: method path => %v", path)
	}
	for _, it := range sd.Streams {
		desc := it
		path := fmt.Sprintf("%v.%v", prefix, desc.StreamName)
		p.handlers[path] = serverStreamHandler(ss, desc.Handler)
		p.log.Infof("RegisterService: stream path => %v", path)
	}

	p.nc.Flush()
}

func (p *Server) onMessage(msg *nats.Msg) {
	p.log.Infof("Proxy.onMessage: subject %v, replay %v, data %v", msg.Subject, msg.Reply, string(msg.Data))
	method := msg.Subject
	log := p.log.WithField("method", method)

	p.mu.Lock()
	stream, ok := p.streams[msg.Reply]
	if !ok {
		stream = newServerStream(p, method, msg.Reply, log)
		p.streams[msg.Reply] = stream
		go stream.onRequest(msg)
	} else {
		go stream.onMessage(msg)
	}
	p.mu.Unlock()
}

func (p *Server) remove(subj string) {
	p.mu.Lock()
	delete(p.streams, subj)
	p.mu.Unlock()
}

var (
	// https://github.com/grpc/grpc-go/blob/master/internal/transport/http2_server.go#L54

	// ErrIllegalHeaderWrite indicates that setting header is illegal because of
	// the stream's state.
	ErrIllegalHeaderWrite = errors.New("transport: the stream is done or WriteHeader was already called")
)

type serverStream struct {
	ctx       context.Context
	cancel    context.CancelFunc
	proxy     *Server
	log       *logrus.Entry
	recvRead  <-chan []byte
	recvWrite chan<- []byte
	hasBegun  bool
	header    metadata.MD
	trailer   metadata.MD
	method    string
	reply     string
}

func newServerStream(proxy *Server, method, reply string, log *logrus.Entry) *serverStream {
	s := &serverStream{
		proxy:  proxy,
		log:    log,
		method: method,
		reply:  reply,
	}
	s.ctx, s.cancel = context.WithCancel(proxy.ctx)
	recv := make(chan []byte, 1)
	s.recvRead = recv
	s.recvWrite = recv
	return s
}

func (s *serverStream) done() {
	s.cancel()
	s.proxy.remove(s.reply)
}

func (s *serverStream) onRequest(msg *nats.Msg) {
	s.log = s.log.WithField("method", s.method)
	handlerFunc, ok := s.proxy.handlers[s.method]
	if !ok {
		s.close(status.Error(codes.Unimplemented, codes.Unimplemented.String()))
		return
	}
	s.recvWrite <- msg.Data
	go handlerFunc(s)
}

func (s *serverStream) onMessage(msg *nats.Msg) {
	if s.recvWrite == nil {
		s.log.Error("data received after client closeSend")
		return
	}
	s.recvWrite <- msg.Data
}

func (s *serverStream) close(err error) {
	if err != nil {
		st, _ := status.FromError(err)
		s.SendMsg(st.Proto())
	}
	s.done()
}

//
// Server Stream interface
//

func (s *serverStream) SetHeader(header metadata.MD) error {
	if s.hasBegun {
		return ErrIllegalHeaderWrite
	}
	if s.header == nil {
		s.header = header
	} else if header != nil {
		s.header = metadata.Join(s.header, header)
	}
	return nil
}

func (s *serverStream) SendHeader(header metadata.MD) error {
	err := s.SetHeader(header)
	if err != nil {
		return err
	}
	return nil
}

func (s *serverStream) SetTrailer(trailer metadata.MD) {
	if s.trailer == nil {
		s.trailer = trailer
	} else if trailer != nil {
		s.trailer = metadata.Join(s.trailer, trailer)
	}
}

func (s *serverStream) Context() context.Context {
	return s.ctx
}

func (s *serverStream) SendMsg(m interface{}) (err error) {
	defer func() {
		if err != nil {
			s.close(err)
		}
	}()

	data, err := proto.Marshal(m.(proto.Message))
	if err == nil {
		err = s.proxy.nc.Publish(s.reply, data)
	}
	return
}

func (s *serverStream) RecvMsg(m interface{}) error {
	select {
	case <-s.ctx.Done():
		return s.ctx.Err()
	case bytes, ok := <-s.recvRead:
		if ok {
			return proto.Unmarshal(bytes, m.(proto.Message))
		}
		return io.EOF
	}
}
