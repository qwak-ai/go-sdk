package authentication

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/qwak-ai/go-sdk/qwak/http"
	"golang.org/x/sync/singleflight"
	"sync"
	"time"
)

const (
	TokenExpirationBuffer = 30 * time.Minute
	stalenessTokenPeriod  = 2 * time.Hour
)

type Authenticator struct {
	parentCtx     context.Context
	ctx           context.Context
	cancelContext context.CancelFunc
	apiKey        string
	httpClient    http.Client
	singleFlight  singleflight.Group

	lock         sync.Mutex
	tokenWrapper tokenWrapper
}

type AuthenticatorOptions struct {
	// Deprecated: unused
	Ctx        context.Context
	ApiKey     string
	HttpClient http.Client
}

type authResponse struct {
	AccessToken string `json:"accessToken"`
	ExpiredAt   int64  `json:"expiredAt"`
}

type tokenWrapper struct {
	accessToken string
	expiredAt   time.Time
}

func NewAuthenticator(options *AuthenticatorOptions) *Authenticator {

	authenticator := &Authenticator{
		httpClient: options.HttpClient,
		apiKey:     options.ApiKey,
	}

	return authenticator
}

func (a *Authenticator) GetToken(ctx context.Context) (string, error) {
	token := a.token()
	expiredIn := getExpiredIn(token)
	if expiredIn <= 0 {
		newToken, err := a.renewToken(ctx)
		if err != nil {
			return "", err
		}
		token = newToken
	} else if expiredIn < stalenessTokenPeriod {
		a.lazyRenewToken()
	}
	return token.accessToken, nil
}

func (a *Authenticator) token() tokenWrapper {
	a.lock.Lock()
	defer a.lock.Unlock()
	return a.tokenWrapper
}

func (a *Authenticator) lazyRenewToken() {
	go func() {
		_, _, _ = a.singleFlight.Do("token-lazy-renew", func() (interface{}, error) {
			ctx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancelFunc()
			_, err := a.renewToken(ctx)
			if err != nil {
				return nil, err
			}
			return nil, nil
		})
	}()

}

func (a *Authenticator) renewToken(ctx context.Context) (tokenWrapper, error) {

	token, err, _ := a.singleFlight.Do("token-get", func() (interface{}, error) {
		tokenResponse, err := a.doGetTokenRequest(ctx, a.apiKey)

		if err != nil {
			return tokenWrapper{}, err
		}

		a.lock.Lock()
		defer a.lock.Unlock()
		a.tokenWrapper = tokenWrapper{
			accessToken: tokenResponse.AccessToken,
			expiredAt:   time.Unix(tokenResponse.ExpiredAt, 0),
		}
		return a.tokenWrapper, nil

	})

	return token.(tokenWrapper), err
}

func (a *Authenticator) doGetTokenRequest(ctx context.Context, apiKey string) (authResponse, error) {

	decodedResponse := authResponse{}
	request, err := http.GetAuthenticationRequest(ctx, apiKey)

	if err != nil {
		return decodedResponse, err
	}
	body, statusCode, err := http.DoRequestWithRetry(a.httpClient, request, http.RetryPolicy{
		MaxAttempts:              5,
		IntervalMs:               200,
		ExponentialBackoffFactor: 1.5,
	})

	if err != nil {
		return decodedResponse, err
	}

	if statusCode == 401 {
		return decodedResponse, errors.New("wrong apiKey, authentication failed with status code 401")
	}

	if statusCode != 200 {
		return decodedResponse, fmt.Errorf("authentication failed. failed with code %d. response: %s", statusCode, body)
	}

	err = json.Unmarshal(body, &decodedResponse)

	if err != nil {
		return decodedResponse, errors.New("failed to unmarshal authentication response")
	}

	return decodedResponse, nil
}

func getExpiredIn(token tokenWrapper) time.Duration {
	now := time.Now()

	if token.expiredAt.IsZero() {
		return 0
	}

	nextAuthIn := token.expiredAt.Sub(now) - (TokenExpirationBuffer)

	if nextAuthIn < 0 {
		return 0
	}

	return nextAuthIn
}
