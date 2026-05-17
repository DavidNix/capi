package googleads_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/davidnix/capi/googleads"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func TestDiscoveryClient_ListAccessibleCustomers(t *testing.T) {
	t.Parallel()

	requests := make(chan capturedDiscoveryRequest, 1)
	writeErrs := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- capturedDiscoveryRequest{
			Method:         r.Method,
			Path:           r.URL.Path,
			AuthHeader:     r.Header.Get("Authorization"),
			DeveloperToken: r.Header.Get("developer-token"),
		}
		writeErrs <- json.NewEncoder(w).Encode(map[string]any{
			"resourceNames": []string{"customers/1234567890", "customers/1112223333"},
		})
	}))
	defer server.Close()

	client, err := googleads.NewDiscoveryClient(t.Context(), testConfig(), googleads.DiscoveryOptions{
		HTTPClient:  server.Client(),
		BaseURL:     server.URL,
		TokenSource: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"}),
	})
	require.NoError(t, err)

	customers, err := client.ListAccessibleCustomers(t.Context())
	require.NoError(t, err)
	require.Equal(t, []googleads.AccessibleCustomer{
		{ResourceName: "customers/1234567890", CustomerID: "1234567890"},
		{ResourceName: "customers/1112223333", CustomerID: "1112223333"},
	}, customers)
	require.NoError(t, <-writeErrs)

	request := <-requests
	require.Equal(t, http.MethodGet, request.Method)
	require.Equal(t, "/v24/customers:listAccessibleCustomers", request.Path)
	require.Equal(t, "Bearer test-token", request.AuthHeader)
	require.Equal(t, "dev-token", request.DeveloperToken)
}

func TestDiscoveryClient_ListAccessibleCustomersUsesRequestContextForTokenRefresh(t *testing.T) {
	t.Parallel()

	clientHTTP := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Host {
		case "oauth2.googleapis.com":
			require.NoError(t, req.Context().Err())
			return testHTTPResponse(http.StatusOK, `{"access_token":"test-token","token_type":"Bearer","expires_in":3600}`), nil
		case "googleads.example":
			require.Equal(t, "Bearer test-token", req.Header.Get("Authorization"))
			return testHTTPResponse(http.StatusOK, `{"resourceNames":["customers/1234567890"]}`), nil
		default:
			require.Failf(t, "unexpected request host", "host=%s", req.URL.Host)
			return testHTTPResponse(http.StatusInternalServerError, `{}`), nil
		}
	})}
	constructorCtx, cancel := context.WithCancel(t.Context())
	client, err := googleads.NewDiscoveryClient(constructorCtx, testConfig(), googleads.DiscoveryOptions{
		HTTPClient: clientHTTP,
		BaseURL:    "https://googleads.example",
	})
	require.NoError(t, err)
	cancel()

	customers, err := client.ListAccessibleCustomers(t.Context())

	require.NoError(t, err)
	require.Equal(t, []googleads.AccessibleCustomer{{ResourceName: "customers/1234567890", CustomerID: "1234567890"}}, customers)
}

func TestDiscoveryClient_ListConversionActions(t *testing.T) {
	t.Parallel()

	requests := make(chan capturedDiscoveryRequest, 1)
	writeErrs := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		decodeErr := json.NewDecoder(r.Body).Decode(&body)
		requests <- capturedDiscoveryRequest{
			Method:          r.Method,
			Path:            r.URL.Path,
			AuthHeader:      r.Header.Get("Authorization"),
			DeveloperToken:  r.Header.Get("developer-token"),
			LoginCustomerID: r.Header.Get("login-customer-id"),
			Query:           body["query"],
		}
		if decodeErr != nil {
			writeErrs <- decodeErr
			return
		}
		writeErrs <- json.NewEncoder(w).Encode([]map[string]any{
			{
				"results": []map[string]any{
					{
						"conversionAction": map[string]any{
							"resourceName": "customers/1234567890/conversionActions/987654321",
							"id":           "987654321",
							"name":         "Backend Lead",
							"status":       "ENABLED",
							"type":         "UPLOAD_CLICKS",
							"category":     "SUBMIT_LEAD_FORM",
						},
					},
					{
						"conversionAction": map[string]any{
							"resourceName": "customers/1234567890/conversionActions/555555555",
							"id":           "555555555",
							"name":         "Website Lead",
							"status":       "ENABLED",
							"type":         "WEBPAGE",
							"category":     "SUBMIT_LEAD_FORM",
						},
					},
				},
			},
		})
	}))
	defer server.Close()

	cfg := testConfig()
	cfg.LoginCustomerID = "111-222-3333"
	client, err := googleads.NewDiscoveryClient(t.Context(), cfg, googleads.DiscoveryOptions{
		HTTPClient:  server.Client(),
		BaseURL:     server.URL,
		TokenSource: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"}),
	})
	require.NoError(t, err)

	actions, err := client.ListConversionActions(t.Context(), "123-456-7890")
	require.NoError(t, err)
	require.Equal(t, []googleads.ConversionAction{
		{ResourceName: "customers/1234567890/conversionActions/987654321", ID: "987654321", Name: "Backend Lead", Status: "ENABLED", Type: "UPLOAD_CLICKS", Category: "SUBMIT_LEAD_FORM"},
		{ResourceName: "customers/1234567890/conversionActions/555555555", ID: "555555555", Name: "Website Lead", Status: "ENABLED", Type: "WEBPAGE", Category: "SUBMIT_LEAD_FORM"},
	}, actions)
	require.True(t, actions[0].SupportsClickConversions())
	require.False(t, actions[1].SupportsClickConversions())
	require.NoError(t, <-writeErrs)

	request := <-requests
	require.Equal(t, http.MethodPost, request.Method)
	require.Equal(t, "/v24/customers/1234567890/googleAds:searchStream", request.Path)
	require.Equal(t, "Bearer test-token", request.AuthHeader)
	require.Equal(t, "dev-token", request.DeveloperToken)
	require.Equal(t, "1112223333", request.LoginCustomerID)
	require.Contains(t, request.Query, "FROM conversion_action")
	require.Contains(t, request.Query, "conversion_action.type")
}

type capturedDiscoveryRequest struct {
	Method          string
	Path            string
	AuthHeader      string
	DeveloperToken  string
	LoginCustomerID string
	Query           string
}

func TestDiscoveryClient_ListConversionActionsRequiresCustomerID(t *testing.T) {
	t.Parallel()

	client, err := googleads.NewDiscoveryClient(t.Context(), testConfig(), googleads.DiscoveryOptions{TokenSource: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"})})
	require.NoError(t, err)

	_, err = client.ListConversionActions(t.Context(), strings.Repeat(" ", 2))

	require.EqualError(t, err, "google ads customer id is required")
}
