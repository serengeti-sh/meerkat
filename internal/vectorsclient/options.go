package vectorsclient

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type config struct {
	dialOpts []grpc.DialOption
}

// Option configures a Client.
type Option func(*config)

// WithGRPCDialOpts appends custom gRPC dial options.
func WithGRPCDialOpts(opts ...grpc.DialOption) Option {
	return func(c *config) {
		c.dialOpts = append(c.dialOpts, opts...)
	}
}

// WithTransportCredentials sets TLS credentials for the gRPC connection.
// Use this in production to secure traffic between services.
func WithTransportCredentials(creds credentials.TransportCredentials) Option {
	return func(c *config) {
		c.dialOpts = append(c.dialOpts, grpc.WithTransportCredentials(creds))
	}
}
