package httpstatus

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
)

func TestStateHandlerFixture(t *testing.T) {
	gunit.Run(new(StateHandlerFixture), t)
}

type StateHandlerFixture struct {
	*gunit.Fixture
	request  *http.Request
	response *httptest.ResponseRecorder
	handler  http.Handler
}

func (this *StateHandlerFixture) Setup() {
	this.request = httptest.NewRequest(http.MethodGet, "/", nil)
	this.response = httptest.NewRecorder()
	this.handler = newStateHandler(http.StatusTeapot, "APP", "RESOURCE", "STATE", "VERSION")
}

func (this *StateHandlerFixture) handle() {
	this.handler.ServeHTTP(this.response, this.request)
}
func (this *StateHandlerFixture) assertPlainTextResponse(expected string) {
	this.So(this.response.Code, should.Equal, http.StatusTeapot)
	this.So(this.response.Header().Get("Content-Type"), should.Equal, "text/plain")
	this.So(this.response.Body.String(), should.Equal, expected)
}
func (this *StateHandlerFixture) assertJSONResponse() {
	this.So(this.response.Code, should.Equal, http.StatusTeapot)
	this.So(this.response.Header().Get("Content-Type"), should.Equal, "application/json; charset=utf-8")
	var actual jsonResponse
	err := json.Unmarshal(this.response.Body.Bytes(), &actual)
	this.So(err, should.BeNil)
	this.So(actual, should.Equal, jsonResponse{
		Compatibility: "APP:STATE",
		Application:   "APP",
		Resource:      "RESOURCE",
		State:         "STATE",
		Version:       "VERSION",
	})
}

func (this *StateHandlerFixture) TestPlainText_NoVersion() {
	this.request.Header["Accept"] = []string{"text/plain"}
	this.handler = newStateHandler(http.StatusTeapot, "APP", "RESOURCE", "STATE", "")
	this.handle()
	this.assertPlainTextResponse("APP:STATE")
}
func (this *StateHandlerFixture) TestPlainText_WithVersion() {
	this.request.Header["Accept"] = []string{"text/plain"}
	this.handler.ServeHTTP(this.response, this.request)
	this.assertPlainTextResponse("APP:STATE\nversion:VERSION")
}
func (this *StateHandlerFixture) TestJSONFromAbsentAccept() {
	this.request.Header["Accept"] = nil
	this.handler.ServeHTTP(this.response, this.request)
	this.assertJSONResponse()
}
func (this *StateHandlerFixture) TestJSONFromWildcardAccept() {
	this.request.Header["Accept"] = []string{"blah", "*/*"}
	this.handler.ServeHTTP(this.response, this.request)
	this.assertJSONResponse()
}
func (this *StateHandlerFixture) TestJSONFromJSONAccept() {
	this.request.Header["Accept"] = []string{"blah", "blah-blah/json-blah"}
	this.handler.ServeHTTP(this.response, this.request)
	this.assertJSONResponse()
}
