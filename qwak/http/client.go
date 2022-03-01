package http

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
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
	return http.DefaultClient
}

func DoRequest(client Client, request *http.Request) (responseBody []byte, httpCode int, err error) {

	response, err := client.Do(request)

	if err != nil {
		return nil, 0, fmt.Errorf("an error occured on http request: %v", err.Error())
	}
	defer response.Body.Close()
	
	body, err := ioutil.ReadAll(response.Body)

	if err != nil {
		return nil, response.StatusCode, errors.New("failed decode authentication request")
	}

	return body, response.StatusCode, nil

}

func DoRequestWithRetry(client Client, request *http.Request) (responseBody []byte, statusCode int, err error) {
	retryAttempt := 0
	lastHttpCode := 500
	var lastErr error
	var body []byte

	for retryAttempt < MaximumRetryAttempts && lastHttpCode >= 500 && lastErr == nil {
		body, lastHttpCode, lastErr = DoRequest(client, request)

		if lastHttpCode >= 500 || err == nil {
			time.Sleep(RetryDelay * time.Duration(int(math.Pow(2, float64(retryAttempt)))))
		}
	}

	return body, lastHttpCode, nil
}
