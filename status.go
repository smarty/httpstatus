package httpstatus

import (
	"context"
	"errors"
	"io"
	"net/http"
	"sync/atomic"
	"time"
)

type defaultStatus struct {
	monitor
	resourceName  string
	state         *atomic.Uint32
	stateHandlers [4]http.Handler
	hardContext   context.Context
	softContext   context.Context
	shutdown      context.CancelFunc
	healthCheck   HealthCheck
	timeout       time.Duration
	frequency     time.Duration
	delay         time.Duration
	logger        logger
}

func newHandler(config configuration) Handler {
	softContext, softShutdown := context.WithCancel(config.ctx)

	var stateHandlers [4]http.Handler
	stateHandlers[stateStarting] = newStateHandler(http.StatusServiceUnavailable, config.displayName, config.resourceName, config.startingState, config.version)
	stateHandlers[stateStopping] = newStateHandler(http.StatusServiceUnavailable, config.displayName, config.resourceName, config.stoppingState, config.version)
	stateHandlers[stateFailing] = newStateHandler(http.StatusServiceUnavailable, config.displayName, config.resourceName, config.failingState, config.version)
	stateHandlers[stateHealthy] = newStateHandler(http.StatusOK, config.displayName, config.resourceName, config.healthyState, config.version)

	return &defaultStatus{
		monitor:       config.monitor,
		resourceName:  config.resourceName,
		state:         new(atomic.Uint32),
		stateHandlers: stateHandlers,
		hardContext:   config.ctx,
		softContext:   softContext,
		shutdown:      softShutdown,
		healthCheck:   config.healthCheck,
		timeout:       config.healthCheckTimeout,
		frequency:     config.healthCheckFrequency,
		delay:         config.shutdownDelay,
		logger:        config.logger,
	}
}

func (this *defaultStatus) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	this.stateHandlers[this.state.Load()].ServeHTTP(response, request)
}
func (this *defaultStatus) Listen() {
	defer this.Stopping()

	for this.isAlive() && this.checkHealth() {
	}
}
func (this *defaultStatus) checkHealth() bool {
	ctx, cancel := context.WithTimeout(this.softContext, this.timeout)
	defer cancel()
	if err := this.healthCheck.Status(ctx); err == nil {
		this.Healthy()
	} else if errors.Is(err, context.Canceled) {
		return false
	} else {
		this.Failing(err)
	}

	this.awaitNextCheck()
	return true
}

func (this *defaultStatus) awaitNextCheck() {
	ctx, cancel := context.WithTimeout(this.softContext, this.frequency)
	defer cancel()
	<-ctx.Done()
}

func (this *defaultStatus) isAlive() bool {
	select {
	case <-this.softContext.Done():
		return false
	default:
		return true
	}
}
func (this *defaultStatus) Healthy() {
	if this.state.Swap(stateHealthy) == stateHealthy {
		return // state hasn't changed, previously healthy
	}

	this.monitor.Healthy()
	this.logger.Printf("[INFO] Health check for resource [%s] passed.", this.resourceName)
}
func (this *defaultStatus) Failing(err error) {
	if this.state.Swap(stateFailing) == stateFailing {
		return // state hasn't changed, previously failing
	}

	this.monitor.Failing(err)
	this.logger.Printf("[WARN] Health check for resource [%s] failing: [%s].", this.resourceName, err)
}
func (this *defaultStatus) Stopping() {
	previousState := this.state.Swap(stateStopping)
	this.monitor.Stopping()

	if previousState != stateHealthy {
		this.logger.Printf("[INFO] Health check for resource [%s] entering [stopping] state. Skipping [%s] shutdown delay because state is unhealthy.", this.resourceName, this.delay)
		return // already unhealthy, avoid shutdown delay
	}

	this.logger.Printf("[INFO] Health check for resource [%s] entering [stopping] state. Waiting [%s] before concluding.", this.resourceName, this.delay)
	ctx, cancel := context.WithTimeout(this.hardContext, this.delay)
	defer cancel()
	<-ctx.Done()
}

func (this *defaultStatus) Close() error {
	tryClose(this.healthCheck)
	this.shutdown()
	return nil
}

func tryClose(v any) {
	if v == nil {
		return
	}
	closer, ok := v.(io.Closer)
	if !ok {
		return
	}
	_ = closer.Close()
}

const (
	stateStarting = iota
	stateHealthy
	stateFailing
	stateStopping
)
