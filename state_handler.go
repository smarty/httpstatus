package httpstatus

import (
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

func newStateHandler(statusCode int, resource, state, version string) http.Handler {
	message := fmt.Sprintf("%s:%s", resource, state)

	plaintext := fmt.Sprintf("%s\nversion:%s", message, version)
	plaintext = strings.TrimSuffix(plaintext, "version:")
	plaintext = strings.TrimSpace(plaintext)

	rawJSON, _ := json.Marshal(struct {
		State   string `json:"state,omitempty"`
		Version string `json:"version,omitempty"`
	}{
		State:   message,
		Version: version,
	})

	return &stateHandler{
		statusCode: statusCode,
		json:       rawJSON,
		plaintext:  []byte(plaintext),
		value:      plaintext,
	}
}

func (this *stateHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	headers, body := this.resolveContent(request.Header["Accept"])
	response.WriteHeader(this.statusCode)
	response.Header()["Accept"] = headers
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
