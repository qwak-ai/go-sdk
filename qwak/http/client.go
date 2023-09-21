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

func GetDefaultHttpClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   30,
			MaxConnsPerHost:       30,
			IdleConnTimeout:       20 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			ForceAttemptHTTP2:     true,
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
	var lastHttpCode int
	var errs []string
	var lastErr error
	var body []byte

	for retryAttempt := 0; retryAttempt < policy.getMaxAttempts() && (retryAttempt == 0 || lastErr != nil); retryAttempt++ {

		if request.Context().Err() != nil {
			lastErr = request.Context().Err()
			errs = append(errs, fmt.Sprintf("Attempt #%d discarded: %v", retryAttempt, lastErr.Error()))
			break
		} else {
			body, lastHttpCode, lastErr = executeRequest(client, request)
		}

		if lastErr == nil && lastHttpCode >= 500 {
			lastErr = fmt.Errorf("request failed with status code '%d'", lastHttpCode)
		}

		if lastErr != nil {
			if lastErr != nil {
				errs = append(errs, fmt.Sprintf("Attempt #%d: %v", retryAttempt, lastErr.Error()))
			}
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
	// MaxAttempts number of attempts on failure
	MaxAttempts int
	// IntervalMs the duration to wait before retry
	// wait time = IntervalMs * (ExponentialBackoffFactor ^ attempt no.)
	IntervalMs int
	// ExponentialBackoffFactor == 1 - Linear; ExponentialBackoffFactor > 1 - Exponential
	// wait time = IntervalMs * (ExponentialBackoffFactor ^ attempt no.)
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
