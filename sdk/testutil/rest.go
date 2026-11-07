package testutil

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	getRequestTimeout   = 10 * time.Second
	maxGetResponseBytes = 16 << 20
)

// GetRequestWithHeaders defines a wrapper around an HTTP GET request with a provided URL
// and custom headers
// Requests time out after ten seconds and responses may contain at most 16 MiB.
// An error is returned if the request, reading or closing the body fails, or the body exceeds the limit.
func GetRequestWithHeaders(url string, headers map[string]string) (body []byte, err error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: getRequestTimeout}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := res.Body.Close(); err == nil && closeErr != nil {
			body, err = nil, closeErr
		}
	}()

	body, err = io.ReadAll(io.LimitReader(res.Body, maxGetResponseBytes+1))
	if err != nil {
		return nil, err
	}

	if len(body) > maxGetResponseBytes {
		return nil, fmt.Errorf("HTTP response body exceeds %d bytes", maxGetResponseBytes)
	}

	return body, nil
}
