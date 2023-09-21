package qwak

import (
	"context"
	"errors"
	"fmt"
	"github.com/qwak-ai/go-sdk/qwak/authentication"
	"github.com/qwak-ai/go-sdk/qwak/http"
	"time"
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
	RetryPolicy   http.RetryPolicy
}

// RealTimeClientConfig a set of configuration for the RealTimeClient
type RealTimeClientConfig struct {
	// ApiKey Your qwak API key
	ApiKey string
	// Environment the environment name
	Environment string
	// RetryPolicy how to retry predict requests, default to no retry
	RetryPolicy http.RetryPolicy
	// RequestTimeout is the timeout of each http request the client performs
	RequestTimeout time.Duration

	// Deprecated: use PredictWithCtx
	Context context.Context
	// HttpClient override the http client created by the NewRealTimeClient constructor
	HttpClient http.Client
}

// NewRealTimeClient is a constructor to initiate a RealTimeClient using to model predictions
func NewRealTimeClient(options RealTimeClientConfig) (*RealTimeClient, error) {

	if len(options.ApiKey) == 0 {
		return nil, errors.New("api key is missing")
	}

	if len(options.Environment) == 0 {
		return nil, errors.New("environment is missing")
	}

	if options.RequestTimeout == 0 {
		options.RequestTimeout = 5 * time.Second
	}

	if options.HttpClient == nil {
		client := http.GetDefaultHttpClient()
		client.Timeout = options.RequestTimeout
		options.HttpClient = client
	}

	return &RealTimeClient{
		authenticator: authentication.NewAuthenticator(&authentication.AuthenticatorOptions{
			ApiKey:     options.ApiKey,
			HttpClient: options.HttpClient,
		}),
		httpClient:  options.HttpClient,
		environment: options.Environment,
		RetryPolicy: options.RetryPolicy,
	}, nil
}

func getPredictionUrl(environment string, modelId string) string {
	return fmt.Sprintf(PredictionBaseUrlTemplate, environment) +
		fmt.Sprintf(PredictionPathUrlTemplate, modelId)
}

// Predict using to perform an inference on your models hosting in Qwak
func (c *RealTimeClient) Predict(predictionRequest *PredictionRequest) (*PredictionResponse, error) {
	return c.PredictWithCtx(context.Background(), predictionRequest)
}

// PredictWithCtx using to perform an inference on your models hosting in Qwak with context to cancel request
func (c *RealTimeClient) PredictWithCtx(ctx context.Context, predictionRequest *PredictionRequest) (*PredictionResponse, error) {
	if len(predictionRequest.modelId) == 0 {
		return nil, errors.New("model id is missing in request")
	}

	token, err := c.authenticator.GetToken(ctx)

	if err != nil {
		return nil, fmt.Errorf("qwak client failed to predict: %s", err.Error())
	}

	pandaOrientedDf := predictionRequest.asPandaOrientedDf()
	predictionUrl := getPredictionUrl(c.environment, predictionRequest.modelId)
	request, err := http.GetPredictionRequest(ctx, predictionUrl, token, pandaOrientedDf)

	if err != nil {
		return nil, fmt.Errorf("qwak client failed to predict: %s", err.Error())
	}

	responseBody, statusCode, err := http.DoRequestWithRetry(c.httpClient, request, c.RetryPolicy)

	if err != nil {
		return nil, fmt.Errorf("qwak client failed to send predict request: %w", err)
	}

	if statusCode != 200 {
		return nil, fmt.Errorf("qwak prediction failed - model respond with status code %d. response: %s", statusCode, responseBody)
	}

	response, err := responseFromRaw(responseBody)

	if err != nil {
		return nil, fmt.Errorf("qwak client failed to parse response from model: %s", err.Error())
	}

	return response, nil
}
