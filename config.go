package httpstatus

import (
	"context"
	"time"
)

type configuration struct {
	displayName          string
	resourceName         string
	startingState        string
	healthyState         string
	failingState         string
	stoppingState        string
	version              string
	healthCheck          HealthCheck
	healthCheckFrequency time.Duration
	healthCheckTimeout   time.Duration
	shutdownDelay        time.Duration
	ctx                  context.Context
	monitor              monitor
	logger               logger
}

func New(options ...option) Handler {
	var config configuration
	Options.apply(options...)(&config)
	return newHandler(config)
}

var Options singleton

type singleton struct{}
type option func(*configuration)

func (singleton) DisplayName(value string) option {
	return func(this *configuration) { this.displayName = value }
}
func (singleton) ResourceName(value string) option {
	return func(this *configuration) { this.resourceName = value }
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
func (singleton) HealthCheckFunc(value healthCheckFunc) option {
	return Options.HealthCheck(NewSimpleHealthCheck(value))
}
func (singleton) SQLHealthCheck(value PingContext) option {
	return Options.HealthCheck(NewPingHealthCheck(value))
}
func (singleton) HealthCheck(value HealthCheck) option {
	return func(this *configuration) { this.healthCheck = value }
}
func (singleton) HealthChecks(all ...HealthCheck) option {
	return Options.HealthCheck(NewCompositeHealthCheck(all...))
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
func (singleton) Logger(value logger) option {
	return func(this *configuration) { this.logger = value }
}
func (singleton) Monitor(value monitor) option {
	return func(this *configuration) { this.monitor = value }
}
func (singleton) VersionName(value string) option {
	return func(this *configuration) { this.version = value }
}

func (singleton) apply(options ...option) option {
	return func(this *configuration) {
		for _, item := range Options.defaults(options...) {
			item(this)
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
	const defaultVersion = ""
	var defaultContext = context.Background()
	var defaultHealthCheck = nop{}
	var defaultMonitor = nop{}
	var defaultLogger = nop{}

	return append([]option{
		Options.DisplayName(defaultName),
		Options.ResourceName(defaultName),
		Options.StartingStateValue(defaultStartingState),
		Options.HealthyStateValue(defaultHealthyState),
		Options.FailingStateValue(defaultFailingState),
		Options.StoppingStateValue(defaultStoppingState),
		Options.HealthCheckFrequency(defaultHealthCheckFrequency),
		Options.HealthCheckTimeout(defaultHealthCheckTimeout),
		Options.ShutdownDelay(defaultShutdownDelay),
		Options.VersionName(defaultVersion),
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

func (nop) Status(_ context.Context) error { return nil }

func (nop) Starting()       {}
func (nop) Healthy()        {}
func (nop) Failing(_ error) {}
func (nop) Stopping()       {}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

type simpleHealthCheck struct{ check healthCheckFunc }

func NewSimpleHealthCheck(check healthCheckFunc) HealthCheck { return &simpleHealthCheck{check: check} }

func (this simpleHealthCheck) Status(ctx context.Context) error { return this.check(ctx) }

type pingHealthCheck struct{ ping PingContext }

func NewPingHealthCheck(ping PingContext) HealthCheck { return &pingHealthCheck{ping: ping} }

func (this pingHealthCheck) Status(ctx context.Context) error { return this.ping.PingContext(ctx) }

type compositeHealthCheck []HealthCheck

func NewCompositeHealthCheck(checks ...HealthCheck) HealthCheck {
	return compositeHealthCheck(checks)
}

func (this compositeHealthCheck) Status(ctx context.Context) error {
	for _, item := range this {
		if err := item.Status(ctx); err != nil {
			return err
		}
	}
	return nil
}
func (this compositeHealthCheck) Close() error {
	for _, item := range this {
		tryClose(item)
	}
	return nil
}
