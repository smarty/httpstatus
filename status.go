package httpstatus

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

type defaultStatus struct {
	monitor
	resourceName string
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
	logger       logger
}

func newHandler(config configuration) Handler {
	softContext, softShutdown := context.WithCancel(config.ctx)

	return &defaultStatus{
		monitor:      config.monitor,
		resourceName: config.resourceName,
		startingText: statusText(config, config.startingState),
		healthyText:  statusText(config, config.healthyState),
		failingText:  statusText(config, config.failingState),
		stoppingText: statusText(config, config.stoppingState),
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

func statusText(config configuration, state string) string {
	result := fmt.Sprintf("%s:%s\nversion:%s", config.displayName, state, config.version)
	return strings.TrimSpace(strings.TrimSuffix(result, "version:"))
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
