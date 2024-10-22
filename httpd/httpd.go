package httpd

import "context"

// HTTPd is the interface for usra to provide the HTTP daemon.
type HTTPd interface {
	Serve(ctx context.Context, addr string) error
}
