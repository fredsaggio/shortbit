//go:build integration

package integration_test

import (
	"context"
	"errors"

	"github.com/fredsaggio/url-shortener/internal/services"
)

type unusedGoogleOIDCClient struct{}

func (unusedGoogleOIDCClient) AuthorizationURL(_, _, _ string) string {
	return "https://accounts.google.com/o/oauth2/v2/auth"
}

func (unusedGoogleOIDCClient) ExchangeAndVerify(context.Context, string, string, string) (services.GoogleIdentity, error) {
	return services.GoogleIdentity{}, errors.New("unexpected Google OIDC call in integration test")
}
