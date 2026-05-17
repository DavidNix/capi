package googleads_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/davidnix/capi/googleads"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"google.golang.org/api/googleapi"
)

type capturedRequest struct {
	Method          string
	AuthHeader      string
	DeveloperToken  string
	LoginCustomerID string
	Body            map[string]any
}

func TestHashEmail(t *testing.T) {
	t.Parallel()

	require.Equal(t, googleads.HashEmail("test@example.com"), googleads.HashEmail(" Test@Example.COM "))
	require.Equal(t, "973dfe463ec85785f5f95af5ba3906eedb2d931c24e69824a89ea65dba4e813b", googleads.HashEmail("test@example.com"))
	require.Equal(t, "ed6c35e905a6daa500da471136a733ccccf446b294e80ae03a1d5d865fe1044c", googleads.HashEmail(" First.Last@Gmail.Com "))
}

func TestClient_UploadLeadConversionUsesRequestContextForTokenRefresh(t *testing.T) {
	t.Parallel()

	clientHTTP := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Host {
		case "oauth2.googleapis.com":
			require.NoError(t, req.Context().Err())
			return testHTTPResponse(http.StatusOK, `{"access_token":"test-token","token_type":"Bearer","expires_in":3600}`), nil
		case "googleads.example":
			require.Equal(t, "Bearer test-token", req.Header.Get("Authorization"))
			return testHTTPResponse(http.StatusOK, `{}`), nil
		default:
			require.Failf(t, "unexpected request host", "host=%s", req.URL.Host)
			return testHTTPResponse(http.StatusInternalServerError, `{}`), nil
		}
	})}
	constructorCtx, cancel := context.WithCancel(t.Context())
	client, err := googleads.NewClient(constructorCtx, testConfig(), googleads.ClientOptions{
		HTTPClient: clientHTTP,
		Endpoint:   "https://googleads.example/upload",
	})
	require.NoError(t, err)
	cancel()

	err = client.UploadLeadConversion(t.Context(), googleads.LeadConversionParams{Email: "test@example.com", ConversionActionID: testConversionActionID, GCLID: "test-gclid"})

	require.NoError(t, err)
}

func TestConfig_Validate(t *testing.T) {
	t.Parallel()

	t.Run("requires fields without naming env vars", func(t *testing.T) {
		err := (googleads.Config{}).Validate()

		require.EqualError(t, err, "google ads configuration is missing CustomerID, DeveloperToken, OAuthClientID, OAuthClientSecret, OAuthRefreshToken")
	})

	t.Run("accepts required fields", func(t *testing.T) {
		err := testConfig().Validate()

		require.NoError(t, err)
	})
}

func TestClient_UploadLeadConversion(t *testing.T) {
	t.Parallel()

	t.Run("uploads lead conversion without value or currency", func(t *testing.T) {
		captures := make(chan capturedRequest, 1)
		writeErrs := make(chan error, 1)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var requestBody map[string]any
			decodeErr := json.NewDecoder(r.Body).Decode(&requestBody)
			captures <- capturedRequest{
				Method:          r.Method,
				AuthHeader:      r.Header.Get("Authorization"),
				DeveloperToken:  r.Header.Get("developer-token"),
				LoginCustomerID: r.Header.Get("login-customer-id"),
				Body:            requestBody,
			}
			if decodeErr != nil {
				writeErrs <- decodeErr
				return
			}
			_, err := w.Write([]byte(`{"results":[{}]}`))
			writeErrs <- err
		}))
		defer server.Close()

		client, err := googleads.NewClient(t.Context(), testConfig(), googleads.ClientOptions{
			HTTPClient:  server.Client(),
			Endpoint:    server.URL,
			TokenSource: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"}),
			Now:         func() time.Time { return time.Date(2026, 5, 9, 15, 4, 5, 0, time.UTC) },
		})
		require.NoError(t, err)

		err = client.UploadLeadConversion(t.Context(), googleads.LeadConversionParams{
			Email:              " Test@Example.COM ",
			ConversionActionID: testConversionActionID,
			GCLID:              "test-gclid_123",
		})
		require.NoError(t, err)
		capture := <-captures
		require.NoError(t, <-writeErrs)

		require.Equal(t, http.MethodPost, capture.Method)
		require.Equal(t, "Bearer test-token", capture.AuthHeader)
		require.Equal(t, "dev-token", capture.DeveloperToken)
		require.Empty(t, capture.LoginCustomerID)
		require.True(t, capture.Body["partialFailure"].(bool))
		require.NotContains(t, capture.Body, "validateOnly")

		conversions := capture.Body["conversions"].([]any)
		require.Len(t, conversions, 1)
		conversion := conversions[0].(map[string]any)
		require.Equal(t, "customers/1234567890/conversionActions/987654321", conversion["conversionAction"])
		require.Equal(t, "2026-05-09 15:04:05+00:00", conversion["conversionDateTime"])
		require.Equal(t, "test-gclid_123", conversion["gclid"])
		require.NotContains(t, conversion, "conversionValue")
		require.NotContains(t, conversion, "currencyCode")

		identifiers := conversion["userIdentifiers"].([]any)
		require.Len(t, identifiers, 1)
		identifier := identifiers[0].(map[string]any)
		require.Equal(t, googleads.HashEmail("test@example.com"), identifier["hashedEmail"])
	})

	t.Run("validates with value currency and login customer id", func(t *testing.T) {
		captures := make(chan capturedRequest, 1)
		writeErrs := make(chan error, 1)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var requestBody map[string]any
			decodeErr := json.NewDecoder(r.Body).Decode(&requestBody)
			captures <- capturedRequest{
				Method:          r.Method,
				AuthHeader:      r.Header.Get("Authorization"),
				DeveloperToken:  r.Header.Get("developer-token"),
				LoginCustomerID: r.Header.Get("login-customer-id"),
				Body:            requestBody,
			}
			if decodeErr != nil {
				writeErrs <- decodeErr
				return
			}
			_, err := w.Write([]byte(`{}`))
			writeErrs <- err
		}))
		defer server.Close()

		cfg := testConfig()
		cfg.LoginCustomerID = "111-222-3333"
		client, err := googleads.NewClient(t.Context(), cfg, googleads.ClientOptions{
			HTTPClient:  server.Client(),
			Endpoint:    server.URL,
			TokenSource: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"}),
		})
		require.NoError(t, err)

		conversionValue := 0.0
		err = client.ValidateLeadConversion(t.Context(), googleads.LeadConversionParams{
			Email:              "test@example.com",
			ConversionActionID: testConversionActionID,
			GBRAID:             "test-gbraid_123",
			ConversionTime:     time.Date(2026, 5, 9, 15, 4, 5, 0, time.UTC),
			ConversionValue:    &conversionValue,
			CurrencyCode:       " USD ",
		})
		require.NoError(t, err)
		capture := <-captures
		require.NoError(t, <-writeErrs)

		require.Equal(t, "1112223333", capture.LoginCustomerID)
		require.Equal(t, true, capture.Body["validateOnly"])
		conversions := capture.Body["conversions"].([]any)
		conversion := conversions[0].(map[string]any)
		require.Equal(t, "test-gbraid_123", conversion["gbraid"])
		require.InDelta(t, 0.0, conversion["conversionValue"], 0.000001)
		require.Equal(t, "USD", conversion["currencyCode"])
	})

	t.Run("serializes wbraid", func(t *testing.T) {
		captures := make(chan capturedRequest, 1)
		writeErrs := make(chan error, 1)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var requestBody map[string]any
			decodeErr := json.NewDecoder(r.Body).Decode(&requestBody)
			captures <- capturedRequest{Body: requestBody}
			if decodeErr != nil {
				writeErrs <- decodeErr
				return
			}
			_, err := w.Write([]byte(`{}`))
			writeErrs <- err
		}))
		defer server.Close()

		client, err := googleads.NewClient(t.Context(), testConfig(), googleads.ClientOptions{
			HTTPClient:  server.Client(),
			Endpoint:    server.URL,
			TokenSource: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"}),
		})
		require.NoError(t, err)

		err = client.UploadLeadConversion(t.Context(), googleads.LeadConversionParams{Email: "test@example.com", ConversionActionID: testConversionActionID, WBRAID: "test-wbraid_123"})
		require.NoError(t, err)
		capture := <-captures
		require.NoError(t, <-writeErrs)

		conversions := capture.Body["conversions"].([]any)
		conversion := conversions[0].(map[string]any)
		require.Equal(t, "test-wbraid_123", conversion["wbraid"])
		require.NotContains(t, conversion, "gclid")
		require.NotContains(t, conversion, "gbraid")
	})

	t.Run("returns google api errors", func(t *testing.T) {
		writeErrs := make(chan error, 1)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, err := w.Write([]byte(`{"error":{"message":"permission denied","errors":[{"reason":"authorizationError"}]}}`))
			writeErrs <- err
		}))
		defer server.Close()

		client, err := googleads.NewClient(t.Context(), testConfig(), googleads.ClientOptions{
			HTTPClient:  server.Client(),
			Endpoint:    server.URL,
			TokenSource: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"}),
		})
		require.NoError(t, err)

		err = client.UploadLeadConversion(t.Context(), googleads.LeadConversionParams{Email: "test@example.com", ConversionActionID: testConversionActionID, GCLID: "test-gclid"})
		require.NoError(t, <-writeErrs)
		require.Error(t, err)

		var googleErr *googleapi.Error
		require.ErrorAs(t, err, &googleErr)
		require.Equal(t, http.StatusForbidden, googleErr.Code)
		require.Equal(t, "authorizationError", googleErr.Errors[0].Reason)
	})

	t.Run("returns google api errors for malformed google api errors", func(t *testing.T) {
		writeErrs := make(chan error, 1)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, err := w.Write([]byte(`{`))
			writeErrs <- err
		}))
		defer server.Close()

		client, err := googleads.NewClient(t.Context(), testConfig(), googleads.ClientOptions{
			HTTPClient:  server.Client(),
			Endpoint:    server.URL,
			TokenSource: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"}),
		})
		require.NoError(t, err)

		err = client.UploadLeadConversion(t.Context(), googleads.LeadConversionParams{Email: "test@example.com", ConversionActionID: testConversionActionID, GCLID: "test-gclid"})
		require.NoError(t, <-writeErrs)
		require.Error(t, err)

		var googleErr *googleapi.Error
		require.ErrorAs(t, err, &googleErr)
		require.Equal(t, http.StatusForbidden, googleErr.Code)
		require.Equal(t, `{`, googleErr.Body)
	})

	t.Run("returns partial failure errors", func(t *testing.T) {
		writeErrs := make(chan error, 1)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, err := w.Write([]byte(`{"partialFailureError":{"code":3,"message":"conversion action not found"}}`))
			writeErrs <- err
		}))
		defer server.Close()

		client, err := googleads.NewClient(t.Context(), testConfig(), googleads.ClientOptions{
			HTTPClient:  server.Client(),
			Endpoint:    server.URL,
			TokenSource: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"}),
		})
		require.NoError(t, err)

		err = client.UploadLeadConversion(t.Context(), googleads.LeadConversionParams{Email: "test@example.com", ConversionActionID: testConversionActionID, GCLID: "test-gclid"})
		require.NoError(t, <-writeErrs)
		require.EqualError(t, err, "google ads partial failure: conversion action not found")
	})

	t.Run("requires email", func(t *testing.T) {
		client, err := googleads.NewClient(t.Context(), testConfig(), googleads.ClientOptions{TokenSource: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"})})
		require.NoError(t, err)

		err = client.UploadLeadConversion(t.Context(), googleads.LeadConversionParams{GCLID: "test-gclid"})

		require.EqualError(t, err, "google ads lead conversion email is required")
	})

	t.Run("requires conversion action id", func(t *testing.T) {
		client, err := googleads.NewClient(t.Context(), testConfig(), googleads.ClientOptions{TokenSource: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"})})
		require.NoError(t, err)

		err = client.UploadLeadConversion(t.Context(), googleads.LeadConversionParams{Email: "test@example.com", GCLID: "test-gclid"})

		require.EqualError(t, err, "google ads lead conversion conversion action id is required")
	})

	t.Run("requires exactly one click id", func(t *testing.T) {
		client, err := googleads.NewClient(t.Context(), testConfig(), googleads.ClientOptions{TokenSource: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"})})
		require.NoError(t, err)

		err = client.UploadLeadConversion(t.Context(), googleads.LeadConversionParams{Email: "test@example.com", ConversionActionID: testConversionActionID})
		require.EqualError(t, err, "google ads lead conversion requires exactly one of gclid, gbraid, or wbraid, got 0")

		err = client.UploadLeadConversion(t.Context(), googleads.LeadConversionParams{Email: "test@example.com", ConversionActionID: testConversionActionID, GCLID: "test-gclid", GBRAID: "test-gbraid"})
		require.EqualError(t, err, "google ads lead conversion requires exactly one of gclid, gbraid, or wbraid, got 2")
	})

	t.Run("rejects invalid click ids", func(t *testing.T) {
		client, err := googleads.NewClient(t.Context(), testConfig(), googleads.ClientOptions{TokenSource: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"})})
		require.NoError(t, err)

		err = client.UploadLeadConversion(t.Context(), googleads.LeadConversionParams{Email: "test@example.com", ConversionActionID: testConversionActionID, GCLID: "bad value"})

		require.EqualError(t, err, "google ads lead conversion gclid is invalid")
	})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func testHTTPResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

const testConversionActionID = "987654321"

func testConfig() googleads.Config {
	return googleads.Config{
		CustomerID:        "123-456-7890",
		DeveloperToken:    "dev-token",
		OAuthClientID:     "client-id",
		OAuthClientSecret: "client-secret",
		OAuthRefreshToken: "refresh-token",
	}
}
