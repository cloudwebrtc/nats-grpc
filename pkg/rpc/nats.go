package rpc

import (
	"time"

	"github.com/nats-io/nats.go"
	"github.com/sirupsen/logrus"
)

var (
	logger *logrus.Entry
)

func init() {
	logger = logrus.WithField("cli", "")
}

//NatsConn nats connection.
type NatsConn interface {
	Publish(subj string, data []byte) error
	PublishRequest(subj, reply string, data []byte) error
	Request(subj string, data []byte, timeout time.Duration) (*nats.Msg, error)
	ChanSubscribe(subj string, ch chan *nats.Msg) (*nats.Subscription, error)
	SubscribeSync(subj string) (*nats.Subscription, error)
	QueueSubscribe(subj, queue string, cb nats.MsgHandler) (*nats.Subscription, error)
	LastError() error
	Flush() error
}

//SetupConnOptions default conn opts.
func SetupConnOptions(opts []nats.Option) []nats.Option {
	totalWait := 10 * time.Minute
	reconnectDelay := time.Second
	connectTimeout := 5 * time.Second

	opts = append(opts, nats.Timeout(connectTimeout))
	opts = append(opts, nats.ReconnectWait(reconnectDelay))
	opts = append(opts, nats.MaxReconnects(int(totalWait/reconnectDelay)))
	opts = append(opts, nats.DisconnectHandler(func(nc *nats.Conn) {
		if !nc.IsClosed() {
			logger.Infof("Disconnected, will attempt reconnects for %.0fm", totalWait.Minutes())
		}
	}))
	opts = append(opts, nats.ReconnectHandler(func(nc *nats.Conn) {
		logger.Infof("Reconnected [%s]", nc.ConnectedUrl())
	}))
	opts = append(opts, nats.ClosedHandler(func(nc *nats.Conn) {
		if !nc.IsClosed() {
			logger.Errorf("Exiting: no servers available")
		} else {
			logger.Errorf("Exiting")
		}
	}))
	return opts
}
