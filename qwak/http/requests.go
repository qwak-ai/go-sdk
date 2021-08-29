package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	AuthRequestContentType    = "application/json"
	BearerTokenTemplate    = "Bearer %s"
	DefaultAuthEndpointUri = "https://grpc.qwak.ai/api/v1/authentication/qwak-api-key"
)

type AuthenticationBody struct {
	ApiKey string `json:"qwakApiKey"`
}

type PandaOrientedDf struct {
	Columns []string        `json:"columns"`
	Index   []int           `json:"index"`
	Data    [][]interface{} `json:"data"`
}

type PredictionResponse struct {
	Predictions []PredictionResult `json:"predictions"`
}

type PredictionResult struct {
	ValuesMap map[string]interface{} `json:"valuesMap"`
}

func NewPandaOrientedDf(columns []string, index []int, data [][]interface{}) PandaOrientedDf {
	return PandaOrientedDf{
		Columns: columns,
		Index:   index,
		Data:    data,
	}
}

func getPostRequest(ctx context.Context, url string, requestBody []byte) (*http.Request, error) {
	bodyBuffer := bytes.NewBuffer(requestBody)

	request, err := http.NewRequestWithContext(ctx, "POST", url, bodyBuffer)

	if err != nil {
		return nil, err
	}

	request.Header.Set("content-type", AuthRequestContentType)

	return request, nil
}

func GetAuthenticationRequest(ctx context.Context, apiKey string) (*http.Request, error) {
	postBody, _ := json.Marshal(&AuthenticationBody{
		ApiKey: apiKey,
	})

	return getPostRequest(ctx, DefaultAuthEndpointUri, postBody)

}

func GetPredictionRequest(ctx context.Context, url string,  token string, dataFrame PandaOrientedDf) (*http.Request, error) {
	postBody, _ := json.Marshal(dataFrame)
	request, err := getPostRequest(ctx, url, postBody)

	if err != nil {
		return nil, err
	}

	request.Header.Set("authorization", fmt.Sprintf(BearerTokenTemplate, token))

	return request, nil

}
