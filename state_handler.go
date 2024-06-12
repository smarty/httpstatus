package httpstatus

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type stateHandler struct {
	statusCode int
	json       []byte
	plaintext  []byte
	value      string
}

func newStateHandler(statusCode int, application, resource, state, version string) http.Handler {
	compatibleMessage := fmt.Sprintf("%s:%s", application, state)

	plaintext := fmt.Sprintf("%s\nversion:%s", compatibleMessage, version)
	plaintext = strings.TrimSuffix(plaintext, "version:")
	plaintext = strings.TrimSpace(plaintext)

	buffer := bytes.NewBuffer(nil)
	encoder := json.NewEncoder(buffer)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(struct {
		Compatibility string `json:"compatibility,omitempty"`
		Application   string `json:"application,omitempty"`
		Resource      string `json:"resource,omitempty"`
		State         string `json:"state,omitempty"`
		Version       string `json:"version,omitempty"`
	}{
		Compatibility: compatibleMessage,
		Application:   application,
		Resource:      resource,
		State:         state,
		Version:       version,
	})

	return &stateHandler{
		statusCode: statusCode,
		json:       buffer.Bytes(),
		plaintext:  []byte(plaintext),
		value:      plaintext,
	}
}

func (this *stateHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	headers, body := this.resolveContent(request.Header["Accept"])
	response.Header()["Content-Type"] = headers
	response.WriteHeader(this.statusCode)
	_, _ = response.Write(body)
}
func (this *stateHandler) resolveContent(acceptHeaders []string) ([]string, []byte) {
	if canWriteJSON(acceptHeaders) {
		return contentTypeJSON, this.json
	} else {
		return contentTypePlaintext, this.plaintext
	}
}
func canWriteJSON(acceptHeaders []string) bool {
	if len(acceptHeaders) == 0 {
		return true
	}

	for _, value := range acceptHeaders {
		if strings.Contains(value, "*/*") || strings.Contains(value, "/json") {
			return true
		}
	}

	return false
}

var (
	contentTypeJSON      = []string{"pplication/json; charset=utf-8"}
	contentTypePlaintext = []string{"text/plain"}
)
