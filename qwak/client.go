package qwak

import (
	"context"
	"errors"
	"fmt"
	"github.com/qwak-ai/go-sdk/qwak/authentication"
	"github.com/qwak-ai/go-sdk/qwak/http"
)

const (
	PredictionPathUrlTemplate = "/v1/%s/predict"
	PredictionBaseUrlTemplate = "https://models.%s.qwak.ai"
)

// RealTimeClient is a client using to inference Qwak models
type RealTimeClient struct {
	authenticator *authentication.Authenticator
	httpClient    http.Client
	environment   string
	context       context.Context
}

// RealTimeClientConfig a set of configuration for the RealTimeClient
type RealTimeClientConfig struct {
	ApiKey      string
	Environment string
	Context     context.Context
	HttpClient  http.Client
}

// NewRealTimeClient is a constructor to initiate a RealTimeClient using to model predictions
func NewRealTimeClient(options RealTimeClientConfig) (*RealTimeClient, error) {

	if len(options.ApiKey) == 0 {
		return nil, errors.New("api key is missing")
	}

	if len(options.Environment) == 0 {
		return nil, errors.New("environment is missing")
	}

	if options.Context == nil {
		options.Context = context.Background()
	}

	if options.HttpClient == nil {
		options.HttpClient = http.GetDefaultHttpClient()
	}

	return &RealTimeClient{
		authenticator: authentication.NewAuthenticator(&authentication.AuthenticatorOptions{
			ApiKey:     options.ApiKey,
			Ctx:        options.Context,
			HttpClient: options.HttpClient,
		}),
		httpClient:  options.HttpClient,
		context:     options.Context,
		environment: options.Environment,
	}, nil
}

func getPredictionUrl(environment string, modelId string) string {
	return fmt.Sprintf(PredictionBaseUrlTemplate, environment) +
		fmt.Sprintf(PredictionPathUrlTemplate, modelId)
}

// Predict using to perform an inference on your models hosting in Qwak
func (c *RealTimeClient) Predict(predictionRequest *PredictionRequest) (*PredictionResponse, error) {
	if len(predictionRequest.ModelId) == 0 {
		return nil, errors.New("model id is missing in request")
	}

	token, err := c.authenticator.GetToken()

	if err != nil {
		return nil, fmt.Errorf("qwak client failed to predict: %s", err.Error())
	}

	pandaOrientedDf := predictionRequest.asPandaOrientedDf()
	predictionUrl := getPredictionUrl(c.environment, predictionRequest.ModelId)
	request, err := http.GetPredictionRequest(c.context, predictionUrl, token, pandaOrientedDf)

	if err != nil {
		return nil, fmt.Errorf("qwak client failed to predict: %s", err.Error())
	}

	responseBody, statusCode, err := http.DoRequest(c.httpClient, request)

	if err != nil {
		return nil, fmt.Errorf("qwak client failed to predict: %s", err.Error())
	}

	if statusCode != 200 {
		return nil, fmt.Errorf("qwak client failed to predict: response with status code %d. response: %s", statusCode, responseBody)
	}

	response, err := responseFromRaw(responseBody)

	if err != nil {
		return nil, fmt.Errorf("qwak client failed to parse response from model: %s", err.Error())
	}

	return response, nil
}
