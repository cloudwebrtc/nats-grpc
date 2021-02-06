package utils

import (
	"math/rand"
	"time"

	"github.com/cloudwebrtc/nats-grpc/pkg/protos/nrpc"
	"github.com/nats-io/nats.go"
	"google.golang.org/grpc/metadata"
)

// RandInt .
func RandInt(min, max int) int {
	rand.Seed(time.Now().UnixNano())
	if min >= max || min == 0 || max == 0 {
		return max
	}
	return rand.Intn(max-min) + min
}

// GenerateRandomNumber .
func GenerateRandomNumber() int {
	return RandInt(1000000, 9999999)
}

// GenerateRandomBytes .
func GenerateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	// Note that err == nil only if we read len(b) bytes.
	if err != nil {
		return nil, err
	}

	return b, nil
}

// GenerateRandomString .
func GenerateRandomString(n int) (string, error) {
	const letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-"
	bytes, err := GenerateRandomBytes(n)
	if err != nil {
		return "", err
	}
	for i, b := range bytes {
		bytes[i] = letters[b%byte(len(letters))]
	}
	return string(bytes), nil
}

func NewInBox() string {
	return nats.NewInbox()
}

func MakeMetadata(md metadata.MD) *nrpc.Metadata {
	if md == nil || md.Len() == 0 {
		return nil
	}
	result := make(map[string]*nrpc.Strings, md.Len())
	for key, values := range md {
		result[key] = &nrpc.Strings{
			Values: values,
		}
	}
	return &nrpc.Metadata{
		Md: result,
	}
}
