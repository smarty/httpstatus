package httpstatus

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
)

func TestStatusFixture(t *testing.T) {
	gunit.Run(new(StatusFixture), t)
}

type StatusFixture struct {
	*gunit.Fixture

	ctx      context.Context
	shutdown context.CancelFunc
	handler  Handler

	healthyCount  int
	failingCount  int
	failingError  error
	stoppingCount int
	statusCount   int
	statusContext context.Context
	statusError   error
	versionName   string

	shutdownContextOnStatusCheck int

	healthCheckTimeout   time.Duration
	healthCheckFrequency time.Duration
	shutdownDelay        time.Duration

	closed int
}

func (this *StatusFixture) Setup() {
	this.ctx, this.shutdown = context.WithCancel(context.Background())
	this.healthCheckTimeout = time.Second
	this.healthCheckFrequency = time.Millisecond
	this.shutdownDelay = time.Millisecond
	this.initialize()
}
func (this *StatusFixture) initialize() {
	this.handler = New(
		Options.ResourceName("resource-name"),
		Options.DisplayName("display-name"),
		Options.HealthCheck(this),
		Options.Monitor(this),
		Options.Context(this.ctx),
		Options.HealthCheckTimeout(this.healthCheckTimeout),
		Options.HealthCheckFrequency(this.healthCheckFrequency),
		Options.ShutdownDelay(this.shutdownDelay),
		Options.VersionName(this.versionName),
	)
}

func (this *StatusFixture) TestHTTPResponseShouldBeWrittenCorrectly() {
	this.versionName = "version"
	this.initialize()
	this.assertHTTP(stateStarting, 503, "Starting")
	this.assertHTTP(stateHealthy, 200, "OK")
	this.assertHTTP(stateFailing, 503, "Failing")
	this.assertHTTP(stateStopping, 503, "Stopping")
}
func (this *StatusFixture) assertHTTP(state uint32, statusCode int, expectedStatus string) {
	this.handler.(*defaultStatus).state.Store(state)
	response := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/", nil)

	this.handler.ServeHTTP(response, request)

	this.So(response.Code, should.Equal, statusCode)
	this.So(response.Header().Get("Content-Type"), should.Equal, "application/json; charset=utf-8")
	var actual jsonResponse
	err := json.Unmarshal(response.Body.Bytes(), &actual)
	this.So(err, should.BeNil)
	this.So(actual, should.Equal, jsonResponse{
		Compatibility: "display-name:" + expectedStatus,
		Application:   "display-name",
		Resource:      "resource-name",
		State:         expectedStatus,
		Version:       this.versionName,
	})
}

func (this *StatusFixture) TestWhenStatusHealthy_MarkAsHealthy() {
	go func() {
		time.Sleep(time.Millisecond)
		_ = this.handler.Close()
	}()

	this.handler.Listen()

	this.So(this.healthyCount, should.BeGreaterThan, 0)
	this.So(this.failingCount, should.Equal, 0)
}
func (this *StatusFixture) TestWhenContextIsCancelled_ListenExits() {
	this.shutdown()

	this.handler.Listen()

	this.So(this.statusContext, should.BeNil)
	this.So(this.healthyCount, should.Equal, 0)
	this.So(this.failingCount, should.Equal, 0)
	this.So(this.stoppingCount, should.Equal, 1)
}
func (this *StatusFixture) TestWhenStatusFailing_MarkAsFailing() {
	this.statusError = errors.New("")
	this.shutdownContextOnStatusCheck = 2

	this.handler.Listen()

	this.So(this.failingCount, should.Equal, 1)
	this.So(this.failingError, should.Equal, this.statusError)
}
func (this *StatusFixture) TestWhenStatusCheckContextTimesOut_MarkAsFailing() {
	this.healthCheckTimeout = time.Nanosecond
	this.shutdownContextOnStatusCheck = 1
	this.initialize()

	this.handler.Listen()

	this.So(this.statusCount, should.Equal, 2)
	this.So(this.failingCount, should.Equal, 1)
	this.So(this.failingError, should.Resemble, context.DeadlineExceeded)
}
func (this *StatusFixture) TestSleepBetweenHealthChecks() {
	this.healthCheckFrequency = time.Millisecond * 10
	this.shutdownContextOnStatusCheck = 1
	this.initialize()

	started := time.Now().UTC()
	this.handler.Listen()

	this.So(time.Since(started), should.BeGreaterThan, this.healthCheckFrequency)
}

func (this *StatusFixture) TestWhenConsecutiveChecksAreHealthy_OnlyUpdateMonitorOnce() {
	this.shutdownContextOnStatusCheck = 4

	this.handler.Listen()

	this.So(this.healthyCount, should.Equal, 1)
	this.So(this.failingCount, should.Equal, 0)
}
func (this *StatusFixture) TestWhenConsecutiveChecksAreFailing_OnlyUpdateMonitorOnce() {
	this.statusError = errors.New("")
	this.shutdownContextOnStatusCheck = 4

	this.handler.Listen()

	this.So(this.failingCount, should.Equal, 1)
	this.So(this.healthyCount, should.Equal, 0)
}

func (this *StatusFixture) TestWhenShuttingDown_DelayShouldBeUsed() {
	this.shutdownDelay = time.Millisecond * 10
	this.initialize()

	go func() {
		time.Sleep(time.Millisecond)
		_ = this.handler.Close()
	}()

	started := time.Now().UTC()
	this.handler.Listen()

	this.So(time.Since(started), should.BeGreaterThan, this.shutdownDelay)
}
func (this *StatusFixture) TestWhenShuttingDownHard_DelayShouldBeIgnored() {
	this.shutdown()
	this.shutdownDelay = time.Second

	started := time.Now().UTC()
	this.handler.Listen()

	this.So(time.Since(started), should.BeLessThan, this.shutdownDelay)
}
func (this *StatusFixture) TestWhenShuttingDownWhileNotHealthy_DelayShouldBeIgnored() {
	this.shutdownDelay = time.Second
	this.statusError = errors.New("")
	this.initialize()

	_ = this.handler.Close()

	started := time.Now().UTC()
	this.handler.Listen()

	this.So(time.Since(started), should.BeLessThan, time.Millisecond)
}
func (this *StatusFixture) TestWhenShuttingDown_HealthCheckClosed() {
	this.initialize()
	_ = this.handler.Close()
	this.So(this.closed, should.Equal, 1)
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func (this *StatusFixture) Status(ctx context.Context) error {
	this.statusCount++
	this.statusContext = ctx

	if this.shutdownContextOnStatusCheck > 0 && this.statusCount > this.shutdownContextOnStatusCheck {
		this.shutdown()
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return this.statusError
	}
}

func (this *StatusFixture) Healthy() {
	this.healthyCount++
}
func (this *StatusFixture) Failing(err error) {
	this.failingCount++
	this.failingError = err
}
func (this *StatusFixture) Stopping() {
	this.stoppingCount++
}
func (this *StatusFixture) Close() error {
	this.closed++
	return nil
}
