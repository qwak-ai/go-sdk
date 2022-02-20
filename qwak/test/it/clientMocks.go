package it

import (
	"fmt"
	"github.com/stretchr/testify/mock"
	"io"
	"net/http"
	"strings"
	"time"
)

type HttpClientMock struct {
	mock.Mock
}

func (hcm *HttpClientMock) Do(request *http.Request) (*http.Response, error) {
	args := hcm.Mock.MethodCalled("Do", request)

	return args.Get(0).(*http.Response), args.Error(1)
}

func GetAuthResponseWithLongExpiration() string {
	now := time.Now()
	expiration := now.Add(time.Hour * 3)

	return fmt.Sprintf("{\"accessToken\":\"jwt-token\",\"expiredAt\":%d}", expiration.Unix())
}

func GetAuthResponseWithExpiredDate() string {
	expired := time.Now().Add(-1 * time.Minute)
	return fmt.Sprintf("{\"accessToken\":\"jwt-token\",\"expiredAt\":%d}", expired.Unix())
}

func GetPredictionResult() string {
	return "[{\"churn\":1}]"
}

func GetPredictionResultWithArrayOfStrings() string {
	return "[{\"strings\":[\"string1\", \"string2\"]}]"
}

func GetHttpReponse(body string, statusCode int) *http.Response {
	return &http.Response{
		Body:       io.NopCloser(strings.NewReader(body)),
		StatusCode: statusCode,
	}
}
