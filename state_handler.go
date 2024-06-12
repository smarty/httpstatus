package httpstatus

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type stateHandler struct {
	statusCode int
	body       []byte
}

func newStateHandler(statusCode int, application, resource, state, version string) http.Handler {
	response := jsonResponse{
		Compatibility: fmt.Sprintf("%s:%s", application, state),
		Application:   application,
		Resource:      resource,
		State:         state,
		Version:       version,
	}
	body, _ := json.MarshalIndent(response, "", "  ")
	return &stateHandler{statusCode: statusCode, body: body}
}

func (this *stateHandler) ServeHTTP(response http.ResponseWriter, _ *http.Request) {
	response.Header()["Content-Type"] = contentTypeJSON
	response.WriteHeader(this.statusCode)
	_, _ = response.Write(this.body)
}

type jsonResponse struct {
	Compatibility string `json:"compatibility,omitempty"`
	Application   string `json:"application,omitempty"`
	Resource      string `json:"resource,omitempty"`
	State         string `json:"state,omitempty"`
	Version       string `json:"version,omitempty"`
}

var contentTypeJSON = []string{"application/json; charset=utf-8"}
