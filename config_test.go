package httpstatus

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
)

func TestConfigFixture(t *testing.T) {
	gunit.Run(new(ConfigFixture), t)
}

type ConfigFixture struct {
	*gunit.Fixture
	count int
	errs  map[int]error
}

func (this *ConfigFixture) Setup() {
	this.errs = make(map[int]error)
}

func (this *ConfigFixture) Status(ctx context.Context) error {
	this.So(ctx.Value("testing"), should.Equal, this.Name())
	count := this.count
	this.count++
	return this.errs[count]
}

func (this *ConfigFixture) TestWhenSQLDBProvided_UseBuiltInHealthCheck() {
	sqlHandle := &sql.DB{}

	handler := New(Options.SQLHealthCheck(sqlHandle))

	// this panics because the handle isn't connected to a real DB, that's how we know it's working
	this.So(handler.Listen, should.Panic)
}

func (this *ConfigFixture) TestWhenNoDefaultHealthCheckProvided_ItShouldReturnHealthy() {
	handler := New()

	go func() {
		time.Sleep(time.Millisecond)
		_ = handler.Close()
	}()

	this.So(handler.Listen, should.NotPanic)
}

func (this *ConfigFixture) TestCompositeHealthCheck() {
	ctx := context.WithValue(context.Background(), "testing", this.Name())
	boink := errors.New("boink")
	this.errs[1] = boink
	composite := NewCompositeHealthCheck(this, this, this)
	err := composite.Status(ctx)
	this.So(err, should.Equal, boink)
	this.So(this.count, should.Equal, 2)
}
