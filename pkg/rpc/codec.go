package rpc

import (
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

// Codec returns a proxying grpc.Codec with the default protobuf codec as parent.
//
// See CodecWithParent.
func Codec() grpc.Codec {
	return CodecWithParent(&protoCodec{})
}

// CodecWithParent returns a proxying grpc.Codec with a user provided codec as parent.
//
// This codec is *crucial* to the functioning of the proxy. It allows the proxy server to be oblivious
// to the schema of the forwarded messages. It basically treats a gRPC message frame as raw bytes.
// However, if the server handler, or the client caller are not proxy-internal functions it will fall back
// to trying to decode the message using a fallback codec.
func CodecWithParent(fallback grpc.Codec) grpc.Codec {
	return &RawCodec{fallback}
}

type RawCodec struct {
	parentCodec grpc.Codec
}

type Frame struct {
	Payload []byte
}

func (c *RawCodec) Marshal(v interface{}) ([]byte, error) {
	out, ok := v.(*Frame)
	if !ok {
		return c.parentCodec.Marshal(v)
	}
	return out.Payload, nil

}

func (c *RawCodec) Unmarshal(data []byte, v interface{}) error {
	dst, ok := v.(*Frame)
	if !ok {
		return c.parentCodec.Unmarshal(data, v)
	}
	dst.Payload = data
	return nil
}

func (c *RawCodec) String() string {
	return fmt.Sprintf("proxy>%s", c.parentCodec.String())
}

// protoCodec is a Codec implementation with protobuf. It is the default rawCodec for gRPC.
type protoCodec struct{}

func (protoCodec) Marshal(v interface{}) ([]byte, error) {
	return proto.Marshal(v.(proto.Message))
}

func (protoCodec) Unmarshal(data []byte, v interface{}) error {
	return proto.Unmarshal(data, v.(proto.Message))
}

func (protoCodec) String() string {
	return "proto"
}
