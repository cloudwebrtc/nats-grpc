package rpc

import (
	"time"

	"github.com/nats-io/go-nats"
)

//NatsConn nats connection.
type NatsConn interface {
	Publish(subj string, data []byte) error
	PublishRequest(subj, reply string, data []byte) error
	Request(subj string, data []byte, timeout time.Duration) (*nats.Msg, error)
	SubscribeSync(subj string) (*nats.Subscription, error)
	QueueSubscribe(subj, queue string, cb nats.MsgHandler) (*nats.Subscription, error)
	LastError() error
	Flush() error
}
