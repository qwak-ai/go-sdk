package http

import (
	"fmt"
	"io/ioutil"
	"math"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	MaximumRetryAttempts = 5
	RetryDelay           = 500 * time.Millisecond
)

type Client interface {
	Do(request *http.Request) (*http.Response, error)
}

func GetDefaultHttpClient() Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       20 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
		Timeout: 3 * time.Second,
	}
}

func executeRequest(client Client, request *http.Request) (responseBody []byte, httpCode int, err error) {

	response, err := client.Do(request)

	if err != nil {
		return nil, 0, fmt.Errorf("an error occured when http request performed: %w", err)
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)

	if err != nil {
		return nil, response.StatusCode, fmt.Errorf("failed to parse request body: %w", err)
	}

	return body, response.StatusCode, nil

}

func DoRequestWithRetry(client Client, request *http.Request, policy RetryPolicy) (responseBody []byte, statusCode int, err error) {

	lastHttpCode := 500
	var errs []string
	var lastErr error
	var body []byte

	for retryAttempt := 0; retryAttempt < policy.getMaxAttempts() && request.Context().Err() == nil && (lastHttpCode >= 500 || lastErr != nil); retryAttempt++ {
		body, lastHttpCode, lastErr = executeRequest(client, request)
		if lastErr != nil {
			errs = append(errs, fmt.Sprintf("Attempt #%d: %v", retryAttempt, lastErr.Error()))
			duration := time.Duration(policy.getBackoffForAttempt(retryAttempt)) * time.Millisecond

			select {
			case <-request.Context().Done():
			case <-time.After(duration):
			}
		}
	}
	if lastErr != nil {
		return body, lastHttpCode, fmt.Errorf("failed to perform reqesut: %w", joinErrors(errs))
	}
	return body, lastHttpCode, nil

}

func joinErrors(errs []string) error {
	return fmt.Errorf("all attemptes has been failed:[%s]", strings.Join(errs, ";"))
}

type RetryPolicy struct {
	MaxAttempts int
	IntervalMs  int
	// ExponentialBackoffFactor =1 - Linear; > 1 - Exponential
	ExponentialBackoffFactor float64
}

func (r *RetryPolicy) hasRetryPolicy() bool {
	return r.MaxAttempts > 1
}

func (r *RetryPolicy) getBackoffForAttempt(attempt int) int {
	factor := r.ExponentialBackoffFactor
	if factor < 1 {
		factor = 1
	}

	if factor > 3 {
		factor = 3
	}

	backoffMultiplier := int(math.Floor(math.Pow(factor, float64(attempt))))
	return backoffMultiplier * backoffMultiplier
}

func (r *RetryPolicy) getMaxAttempts() int {
	if r.MaxAttempts > 5 {
		return 5
	}

	if r.MaxAttempts < 1 {
		return 1
	}

	return r.MaxAttempts
}

func BasicExponentialBackoffRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:              5,
		IntervalMs:               200,
		ExponentialBackoffFactor: 2,
	}
}
