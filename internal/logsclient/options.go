package logsclient

import "google.golang.org/grpc"

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
