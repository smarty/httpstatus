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

type HealthCheckFunc func(ctx context.Context) error
type HealthCheck interface {
	Status(ctx context.Context) error
}

type Monitor interface {
	Healthy()
	Failing(error)
	Stopping()
}
type Logger interface {
	Printf(format string, args ...interface{})
}
