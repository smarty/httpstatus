package httpstatus

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"
)

type defaultStatus struct {
	monitor
	resourceName  string
	state         uint32
	stateHandlers map[uint32]http.Handler
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

	stateHandlers := map[uint32]http.Handler{
		stateStarting: newStateHandler(http.StatusServiceUnavailable, config.resourceName, config.startingState, config.version),
		stateStopping: newStateHandler(http.StatusServiceUnavailable, config.resourceName, config.stoppingState, config.version),
		stateFailing:  newStateHandler(http.StatusServiceUnavailable, config.resourceName, config.failingState, config.version),
		stateHealthy:  newStateHandler(http.StatusOK, config.resourceName, config.healthyState, config.version),
	}

	return &defaultStatus{
		monitor:       config.monitor,
		resourceName:  config.resourceName,
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
	state := atomic.LoadUint32(&this.state)
	handler := this.stateHandlers[state]
	handler.ServeHTTP(response, request)
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
	} else if err == context.Canceled {
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
	if atomic.SwapUint32(&this.state, stateHealthy) == stateHealthy {
		return // state hasn't changed, previously healthy
	}

	this.monitor.Healthy()
	this.logger.Printf("[INFO] Health check for resource [%s] passed.", this.resourceName)
}
func (this *defaultStatus) Failing(err error) {
	if atomic.SwapUint32(&this.state, stateFailing) == stateFailing {
		return // state hasn't changed, previously failing
	}

	this.monitor.Failing(err)
	this.logger.Printf("[WARN] Health check for resource [%s] failing: [%s].", this.resourceName, err)
}
func (this *defaultStatus) Stopping() {
	previousState := atomic.SwapUint32(&this.state, stateStopping)
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
	this.shutdown()
	return nil
}

const (
	stateStarting = iota
	stateHealthy
	stateFailing
	stateStopping
)
