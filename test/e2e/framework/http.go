package framework

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

// EchoEnv is a structure used to unmarshal the ".k8sEnv" field of "echo" responses.
type EchoEnv struct {
	Namespace string `json:"namespace,omitempty"`
	Pod       string `json:"pod,omitempty"`
}

// EchoResponse is a structure used to unmarshal "echo" responses.
type EchoResponse struct {
	K8sEnv EchoEnv             `json:"k8sEnv,omitempty"`
	Header map[string][]string `json:"header,omitempty"`
	Host   string              `json:"host,omitempty"`
	Method string              `json:"method,omitempty"`
	URI    string              `json:"uri,omitempty"`
}

// XForwardedForContains returns a value indicating whether the "X-Forwarded-For" header contains the specified value.
func (r *EchoResponse) XForwardedForContains(v string) bool {
	for key, val := range r.Header {
		if strings.EqualFold(key, "X-Forwarded-For") {
			for _, ip := range val {
				if ip == v {
					return true
				}
			}
		}
	}
	return false
}

// Request performs a "method" request to the specified host and path, returning the status code and the response's body.
// TODO (@bcustodio) Add support for HTTPS if/when necessary.
func (f *Framework) Request(method, host, path string) (int, string, error) {
	// Build the HTTP request.
	req, err := http.NewRequest(method, fmt.Sprintf("http://%s%s", host, path), nil)
	if err != nil {
		return 0, "", err
	}
	// Perform the request.
	res, err := f.HTTPClient.Do(req)
	if err != nil {
		return 0, "", err
	}
	// Read the response's body.
	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return 0, "", err
	}
	// Close the response's body.
	if err := res.Body.Close(); err != nil {
		return 0, "", err
	}
	return res.StatusCode, string(b), nil
}

// EchoRequest performs a "method" request to the specified host and path, returning the resulting "echo" response or an error.
// TODO (@bcustodio) Add support for HTTPS if/when necessary.
func (f *Framework) EchoRequest(method, host string, port int32, path string, headers map[string]string) (*EchoResponse, error) {
	// Build the HTTP request.
	req, err := http.NewRequest(method, fmt.Sprintf("http://%s:%d%s", host, port, path), nil)
	if err != nil {
		return nil, err
	}
	// Set headers as requested.
	setHeaders(req, headers)
	// Perform the request.
	res, err := f.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	// Unmarshal the response's body.
	echo := &EchoResponse{}
	if err := json.NewDecoder(res.Body).Decode(echo); err != nil {
		return nil, err
	}
	// Close the response's body.
	if err := res.Body.Close(); err != nil {
		return nil, err
	}
	// Return the unmarshalled object.
	return echo, nil
}

// setHeaders sets the specified headers on the provided HTTP request.
// NOTE: The "Host" header is special in that it MUST be set using "req.Host" instead of "req.Header.Set(string, string)".
func setHeaders(req *http.Request, headers map[string]string) {
	for key, val := range headers {
		if strings.EqualFold(key, "Host") {
			req.Host = val
		} else {
			req.Header.Set(key, val)
		}
	}
}
