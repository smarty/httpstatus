package httpstatus

import (
	"database/sql"
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
