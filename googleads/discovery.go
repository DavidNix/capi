package googleads

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/oauth2"
)

const conversionActionQuery = `SELECT
  conversion_action.resource_name,
  conversion_action.id,
  conversion_action.name,
  conversion_action.status,
  conversion_action.type,
  conversion_action.category
FROM conversion_action
ORDER BY conversion_action.name`

// AccessibleCustomer identifies a Google Ads customer visible to the OAuth user.
type AccessibleCustomer struct {
	ResourceName string
	CustomerID   string
}

// ConversionAction describes a Google Ads conversion action relevant to uploads.
type ConversionAction struct {
	ResourceName string
	ID           string
	Name         string
	Status       string
	Type         string
	Category     string
}

// SupportsClickConversions reports whether the action accepts UploadClickConversions requests.
func (a ConversionAction) SupportsClickConversions() bool {
	return a.Type == "UPLOAD_CLICKS"
}

// DiscoveryOptions configures Google Ads account discovery.
type DiscoveryOptions struct {
	// HTTPClient is used for Google Ads and OAuth token requests.
	HTTPClient *http.Client
	// BaseURL overrides the Google Ads API base URL.
	BaseURL string
	// APIVersion overrides the default Google Ads API version.
	APIVersion string
	// TokenSource overrides OAuth refresh-token authentication.
	TokenSource oauth2.TokenSource
}

// DiscoveryClient discovers Google Ads customers and conversion actions.
type DiscoveryClient struct {
	cfg         Config
	baseURL     string
	apiVersion  string
	httpClient  *http.Client
	tokenSource requestTokenSource
}

// NewDiscoveryClient creates a Google Ads account discovery client.
func NewDiscoveryClient(ctx context.Context, cfg Config, opts DiscoveryOptions) (*DiscoveryClient, error) {
	_ = ctx

	cfg.CustomerID = CleanCustomerID(cfg.CustomerID)
	cfg.LoginCustomerID = CleanCustomerID(cfg.LoginCustomerID)
	cfg.DeveloperToken = strings.TrimSpace(cfg.DeveloperToken)
	cfg.OAuthClientID = strings.TrimSpace(cfg.OAuthClientID)
	cfg.OAuthClientSecret = strings.TrimSpace(cfg.OAuthClientSecret)
	cfg.OAuthRefreshToken = strings.TrimSpace(cfg.OAuthRefreshToken)

	if err := validateDiscoveryConfig(cfg); err != nil {
		return nil, err
	}

	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultRequestTimeout}
	}

	var tokenSource requestTokenSource
	if opts.TokenSource != nil {
		tokenSource = staticTokenSource{source: opts.TokenSource}
	}
	if tokenSource == nil {
		tokenSource = &refreshTokenSource{cfg: cfg, httpClient: httpClient}
	}

	baseURL := strings.TrimRight(strings.TrimSpace(opts.BaseURL), "/")
	if baseURL == "" {
		baseURL = googleAdsAPIBaseURL
	}
	apiVersion := strings.TrimSpace(opts.APIVersion)
	if apiVersion == "" {
		apiVersion = defaultAPIVersion
	}

	return &DiscoveryClient{cfg: cfg, baseURL: baseURL, apiVersion: apiVersion, httpClient: httpClient, tokenSource: tokenSource}, nil
}

// ListAccessibleCustomers returns Google Ads customers visible to the OAuth user.
func (c *DiscoveryClient) ListAccessibleCustomers(ctx context.Context) ([]AccessibleCustomer, error) {
	endpoint := fmt.Sprintf("%s/%s/customers:listAccessibleCustomers", c.baseURL, c.apiVersion)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create google ads accessible customers request: %w", err)
	}
	req.Header.Set("developer-token", c.cfg.DeveloperToken)
	if authErr := c.authorize(req); authErr != nil {
		return nil, authErr
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list google ads accessible customers: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := readDiscoveryResponse(resp)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode > 299 {
		return nil, googleAPIError(resp.StatusCode, respBody)
	}

	parsed := struct {
		ResourceNames []string `json:"resourceNames"`
	}{}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("parse google ads accessible customers response: %w", err)
	}

	customers := make([]AccessibleCustomer, 0, len(parsed.ResourceNames))
	for _, resourceName := range parsed.ResourceNames {
		customerID := strings.TrimPrefix(resourceName, "customers/")
		customers = append(customers, AccessibleCustomer{ResourceName: resourceName, CustomerID: customerID})
	}

	return customers, nil
}

// ListConversionActions returns conversion actions for a Google Ads customer.
func (c *DiscoveryClient) ListConversionActions(ctx context.Context, customerID string) ([]ConversionAction, error) {
	customerID = CleanCustomerID(customerID)
	if customerID == "" {
		return nil, fmt.Errorf("google ads customer id is required")
	}

	requestBody, err := json.Marshal(struct {
		Query string `json:"query"`
	}{Query: conversionActionQuery})
	if err != nil {
		return nil, fmt.Errorf("marshal google ads conversion action query: %w", err)
	}

	endpoint := fmt.Sprintf("%s/%s/customers/%s/googleAds:searchStream", c.baseURL, c.apiVersion, customerID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("create google ads conversion actions request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("developer-token", c.cfg.DeveloperToken)
	if c.cfg.LoginCustomerID != "" {
		req.Header.Set("login-customer-id", c.cfg.LoginCustomerID)
	}
	if authErr := c.authorize(req); authErr != nil {
		return nil, authErr
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list google ads conversion actions: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := readDiscoveryResponse(resp)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode > 299 {
		return nil, googleAPIError(resp.StatusCode, respBody)
	}

	var parsed []struct {
		Results []struct {
			ConversionAction ConversionAction `json:"conversionAction"`
		} `json:"results"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("parse google ads conversion actions response: %w", err)
	}

	actions := make([]ConversionAction, 0)
	for _, response := range parsed {
		for _, result := range response.Results {
			actions = append(actions, result.ConversionAction)
		}
	}

	return actions, nil
}

func validateDiscoveryConfig(cfg Config) error {
	missing := make([]string, 0, 4)
	fields := []struct {
		name  string
		value string
	}{
		{name: "DeveloperToken", value: strings.TrimSpace(cfg.DeveloperToken)},
		{name: "OAuthClientID", value: strings.TrimSpace(cfg.OAuthClientID)},
		{name: "OAuthClientSecret", value: strings.TrimSpace(cfg.OAuthClientSecret)},
		{name: "OAuthRefreshToken", value: strings.TrimSpace(cfg.OAuthRefreshToken)},
	}
	for _, field := range fields {
		if field.value == "" {
			missing = append(missing, field.name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("google ads discovery configuration is missing %s", strings.Join(missing, ", "))
	}

	return nil
}

func (c *DiscoveryClient) authorize(req *http.Request) error {
	token, err := c.tokenSource.Token(req.Context())
	if err != nil {
		return fmt.Errorf("get google ads token: %w", err)
	}
	token.SetAuthHeader(req)
	return nil
}

func readDiscoveryResponse(resp *http.Response) ([]byte, error) {
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, responseMaxSize))
	if err != nil {
		return nil, fmt.Errorf("read google ads discovery response: %w", err)
	}

	return respBody, nil
}
