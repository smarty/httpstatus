package httpstatus

import (
	"context"
	"database/sql"
	"time"
)

type configuration struct {
	name                 string
	startingState        string
	healthyState         string
	failingState         string
	stoppingState        string
	healthCheck          HealthCheck
	healthCheckFrequency time.Duration
	healthCheckTimeout   time.Duration
	shutdownDelay        time.Duration
	ctx                  context.Context
	monitor              Monitor
	logger               Logger
}

func New(options ...option) Handler {
	var config configuration
	Options.apply(options...)(&config)
	return newHandler(config)
}

var Options singleton

type singleton struct{}
type option func(*configuration)

func (singleton) Name(value string) option {
	return func(this *configuration) { this.name = value }
}
func (singleton) StartingStateValue(value string) option {
	return func(this *configuration) { this.startingState = value }
}
func (singleton) HealthyStateValue(value string) option {
	return func(this *configuration) { this.healthyState = value }
}
func (singleton) FailingStateValue(value string) option {
	return func(this *configuration) { this.failingState = value }
}
func (singleton) StoppingStateValue(value string) option {
	return func(this *configuration) { this.stoppingState = value }
}
func (singleton) HealthCheckFunc(value HealthCheckFunc) option {
	return Options.HealthCheck(defaultHealthCheck{check: value})
}
func (singleton) SQLHealthCheck(value *sql.DB) option {
	return Options.HealthCheck(sqlHealthCheck{DB: value})
}
func (singleton) HealthCheck(value HealthCheck) option {
	return func(this *configuration) { this.healthCheck = value }
}
func (singleton) HealthCheckFrequency(value time.Duration) option {
	return func(this *configuration) { this.healthCheckFrequency = value }
}
func (singleton) HealthCheckTimeout(value time.Duration) option {
	return func(this *configuration) { this.healthCheckTimeout = value }
}
func (singleton) ShutdownDelay(value time.Duration) option {
	return func(this *configuration) { this.shutdownDelay = value }
}
func (singleton) Context(value context.Context) option {
	return func(this *configuration) { this.ctx = value }
}
func (singleton) Logger(value Logger) option {
	return func(this *configuration) { this.logger = value }
}
func (singleton) Monitor(value Monitor) option {
	return func(this *configuration) { this.monitor = value }
}

func (singleton) apply(options ...option) option {
	return func(this *configuration) {
		for _, option := range Options.defaults(options...) {
			option(this)
		}
	}
}
func (singleton) defaults(options ...option) []option {
	const defaultName = "status"
	const defaultStartingState = "Starting"
	const defaultHealthyState = "OK"
	const defaultFailingState = "Failing"
	const defaultStoppingState = "Stopping"
	const defaultHealthCheckFrequency = time.Second
	const defaultHealthCheckTimeout = time.Second * 10
	const defaultShutdownDelay = 0
	var defaultContext = context.Background()
	var defaultHealthCheck = nop{}
	var defaultMonitor = nop{}
	var defaultLogger = nop{}

	return append([]option{
		Options.Name(defaultName),
		Options.StartingStateValue(defaultStartingState),
		Options.HealthyStateValue(defaultHealthyState),
		Options.FailingStateValue(defaultFailingState),
		Options.StoppingStateValue(defaultStoppingState),
		Options.HealthCheckFrequency(defaultHealthCheckFrequency),
		Options.HealthCheckTimeout(defaultHealthCheckTimeout),
		Options.ShutdownDelay(defaultShutdownDelay),
		Options.Context(defaultContext),
		Options.HealthCheckFunc(defaultHealthCheck.Status),
		Options.HealthCheck(defaultHealthCheck),
		Options.Monitor(defaultMonitor),
		Options.Logger(defaultLogger),
	}, options...)
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

type nop struct{}

func (nop) Printf(_ string, _ ...interface{}) {}
func (nop) Println(_ ...interface{})          {}

func (nop) Status(ctx context.Context) error { return ctx.Err() }

func (nop) Starting()       {}
func (nop) Healthy()        {}
func (nop) Failing(_ error) {}
func (nop) Stopping()       {}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

type defaultHealthCheck struct{ check HealthCheckFunc }

func (this defaultHealthCheck) Status(ctx context.Context) error { return this.check(ctx) }

type sqlHealthCheck struct{ *sql.DB }

func (this sqlHealthCheck) Status(ctx context.Context) error { return this.PingContext(ctx) }
