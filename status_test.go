package httpstatus

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/smartystreets/assertions/should"
	"github.com/smartystreets/gunit"
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

	shutdownContextOnStatusCheck int

	healthCheckTimeout   time.Duration
	healthCheckFrequency time.Duration
	shutdownDelay        time.Duration
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
		Options.HealthCheckFunc(this.Status),
		Options.Monitor(this),
		Options.Context(this.ctx),
		Options.HealthCheckTimeout(this.healthCheckTimeout),
		Options.HealthCheckFrequency(this.healthCheckFrequency),
		Options.ShutdownDelay(this.shutdownDelay),
	)
}

func (this *StatusFixture) TestHTTPResponseShouldBeWrittenCorrectly() {
	this.assertHTTP(stateStarting, 503, "status:Starting")
	this.assertHTTP(stateHealthy, 200, "status:OK")
	this.assertHTTP(stateFailing, 503, "status:Failing")
	this.assertHTTP(stateStopping, 503, "status:Stopping")
}
func (this *StatusFixture) assertHTTP(state uint32, statusCode int, responseText string) {
	response := httptest.NewRecorder()
	this.handler.(*defaultStatus).state = state

	this.handler.ServeHTTP(response, nil)

	this.So(response.Code, should.Equal, statusCode)
	this.So(response.Body.String(), should.Equal, responseText+"\n")
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
func (this *StatusFixture) TestWhenContextIsCancelled_ListenExists() {
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
