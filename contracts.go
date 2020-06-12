package httpstatus

import (
	"context"
	"io"
	"net/http"
)

type Handler interface {
	http.Handler

	Listen()
	io.Closer
}

type HealthCheck interface {
	Status(ctx context.Context) error
}
type healthCheckFunc func(ctx context.Context) error

type monitor interface {
	Healthy()
	Failing(error)
	Stopping()
}
type logger interface {
	Printf(format string, args ...interface{})
}

type PingContext interface {
	PingContext(ctx context.Context) error
}