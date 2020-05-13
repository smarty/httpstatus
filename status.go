package httpstatus

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

type defaultStatus struct {
	Monitor
	startingText string
	healthyText  string
	failingText  string
	stoppingText string
	state        uint32
	hardContext  context.Context
	softContext  context.Context
	shutdown     context.CancelFunc
	healthCheck  HealthCheck
	timeout      time.Duration
	frequency    time.Duration
	delay        time.Duration
	logger       Logger
}

func newHandler(config configuration) Handler {
	softContext, softShutdown := context.WithCancel(config.ctx)

	return &defaultStatus{
		Monitor:      config.monitor,
		startingText: fmt.Sprintf("%s:%s", config.name, config.startingState),
		healthyText:  fmt.Sprintf("%s:%s", config.name, config.healthyState),
		failingText:  fmt.Sprintf("%s:%s", config.name, config.failingState),
		stoppingText: fmt.Sprintf("%s:%s", config.name, config.stoppingState),
		hardContext:  config.ctx,
		softContext:  softContext,
		shutdown:     softShutdown,
		healthCheck:  config.healthCheck,
		timeout:      config.healthCheckTimeout,
		frequency:    config.healthCheckFrequency,
		delay:        config.shutdownDelay,
		logger:       config.logger,
	}
}

func (this *defaultStatus) ServeHTTP(response http.ResponseWriter, _ *http.Request) {
	switch atomic.LoadUint32(&this.state) {
	case stateStarting:
		http.Error(response, this.startingText, http.StatusServiceUnavailable)
	case stateFailing:
		http.Error(response, this.failingText, http.StatusServiceUnavailable)
	case stateStopping:
		http.Error(response, this.stoppingText, http.StatusServiceUnavailable)
	default:
		http.Error(response, this.healthyText, http.StatusOK)
	}
}
func (this *defaultStatus) Listen() {
	defer this.Stopping()

	for {
		ctx, _ := context.WithTimeout(this.softContext, this.timeout)
		if err := this.healthCheck.Status(ctx); err == nil {
			this.Healthy()
		} else if err == context.Canceled {
			break
		} else {
			this.Failing(err)
		}

		ctx, _ = context.WithTimeout(this.softContext, this.frequency)
		<-ctx.Done()
	}
}

func (this *defaultStatus) Healthy() {
	if atomic.SwapUint32(&this.state, stateHealthy) == stateHealthy {
		return // state hasn't changed, previously healthy
	}

	this.Monitor.Healthy()
	this.logger.Printf("[INFO] Health check passed.")
}
func (this *defaultStatus) Failing(err error) {
	if atomic.SwapUint32(&this.state, stateFailing) == stateFailing {
		return // state hasn't changed, previously failing
	}

	this.Monitor.Failing(err)
	this.logger.Printf("[WARN] Health check failing: [%s].", err)
}
func (this *defaultStatus) Stopping() {
	atomic.StoreUint32(&this.state, stateStopping)
	this.Monitor.Stopping()
	this.logger.Printf("[INFO] Entering [stopping] state. Waiting [%s] before concluding.", this.delay)

	ctx, _ := context.WithTimeout(this.hardContext, this.delay)
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
