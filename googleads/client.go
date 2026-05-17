package googleads

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/googleapi"
)

const (
	defaultAPIVersion      = "v24"
	adwordsScope           = "https://www.googleapis.com/auth/adwords"
	googleAuthURL          = "https://accounts.google.com/o/oauth2/auth"
	googleTokenURL         = "https://oauth2.googleapis.com/token"
	defaultRequestTimeout  = 10 * time.Second
	responseMaxSize        = 1 << 20
	conversionTimeLayout   = "2006-01-02 15:04:05-07:00"
	googleAdsAPIBaseURL    = "https://googleads.googleapis.com"
	requiredClickIDMessage = "google ads lead conversion requires exactly one of gclid, gbraid, or wbraid, got %d"
)

// Config contains Google Ads conversion upload settings.
type Config struct {
	// CustomerID is the Google Ads customer ID that owns the conversion action.
	CustomerID string
	// DeveloperToken is the Google Ads developer token.
	DeveloperToken string
	// OAuthClientID is the OAuth client ID used to refresh access tokens.
	OAuthClientID string
	// OAuthClientSecret is the OAuth client secret used to refresh access tokens.
	OAuthClientSecret string
	// OAuthRefreshToken is the refresh token with the adwords OAuth scope.
	OAuthRefreshToken string
	// LoginCustomerID is the optional manager account customer ID.
	LoginCustomerID string
}

// ClientOptions configures the Google Ads client.
type ClientOptions struct {
	// HTTPClient is used for Google Ads and OAuth token requests.
	HTTPClient *http.Client
	// Endpoint overrides the full uploadClickConversions endpoint.
	Endpoint string
	// APIVersion overrides the default Google Ads API version.
	APIVersion string
	// TokenSource overrides OAuth refresh-token authentication.
	TokenSource oauth2.TokenSource
	// Now overrides the clock used for default conversion times.
	Now func() time.Time
}

// Client uploads offline conversions to Google Ads.
type Client struct {
	cfg         Config
	endpoint    string
	httpClient  *http.Client
	tokenSource requestTokenSource
	now         func() time.Time
}

type requestTokenSource interface {
	Token(ctx context.Context) (*oauth2.Token, error)
}

type staticTokenSource struct {
	source oauth2.TokenSource
}

func (s staticTokenSource) Token(_ context.Context) (*oauth2.Token, error) {
	return s.source.Token()
}

type refreshTokenSource struct {
	cfg        Config
	httpClient *http.Client

	mu    sync.Mutex
	token *oauth2.Token
}

// LeadConversionParams contains the data needed to upload one lead conversion.
type LeadConversionParams struct {
	// Email is normalized and hashed for enhanced conversions.
	Email string
	// ConversionActionID identifies the Google Ads conversion action for this event.
	ConversionActionID string
	// GCLID is a Google click ID. Exactly one click ID field must be set.
	GCLID string
	// GBRAID is a Google GBRAID click ID. Exactly one click ID field must be set.
	GBRAID string
	// WBRAID is a Google WBRAID click ID. Exactly one click ID field must be set.
	WBRAID string
	// ConversionTime is the time the conversion happened. Zero means current UTC time.
	ConversionTime time.Time
	// ConversionValue is optional and omitted when nil.
	ConversionValue *float64
	// CurrencyCode is optional and omitted when empty.
	CurrencyCode string
}

type clickConversion struct {
	ConversionAction   string           `json:"conversionAction"`
	ConversionDateTime string           `json:"conversionDateTime"`
	ConversionValue    *float64         `json:"conversionValue,omitempty"`
	CurrencyCode       string           `json:"currencyCode,omitempty"`
	GCLID              string           `json:"gclid,omitempty"`
	GBRAID             string           `json:"gbraid,omitempty"`
	WBRAID             string           `json:"wbraid,omitempty"`
	UserIdentifiers    []UserIdentifier `json:"userIdentifiers"`
}

// UserIdentifier contains a hashed user identifier for enhanced conversions.
type UserIdentifier struct {
	// HashedEmail is a SHA-256 hash of a normalized email address.
	HashedEmail string `json:"hashedEmail"`
}

type conversionPayload struct {
	Conversions    []clickConversion `json:"conversions"`
	PartialFailure bool              `json:"partialFailure"`
	ValidateOnly   bool              `json:"validateOnly,omitempty"`
}

type uploadResponse struct {
	PartialFailureError struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"partialFailureError"`
}

// PartialFailureError indicates Google Ads accepted the request but rejected at least one conversion.
type PartialFailureError struct {
	// Message contains the partial failure message returned by Google Ads.
	Message string
}

// Error returns the Google Ads partial failure message.
func (e PartialFailureError) Error() string {
	return "google ads partial failure: " + e.Message
}

// NewClient creates a Google Ads conversion upload client.
func NewClient(ctx context.Context, cfg Config, opts ClientOptions) (*Client, error) {
	_ = ctx

	cfg.CustomerID = CleanCustomerID(cfg.CustomerID)
	cfg.LoginCustomerID = CleanCustomerID(cfg.LoginCustomerID)
	cfg.DeveloperToken = strings.TrimSpace(cfg.DeveloperToken)
	cfg.OAuthClientID = strings.TrimSpace(cfg.OAuthClientID)
	cfg.OAuthClientSecret = strings.TrimSpace(cfg.OAuthClientSecret)
	cfg.OAuthRefreshToken = strings.TrimSpace(cfg.OAuthRefreshToken)

	if err := cfg.Validate(); err != nil {
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

	endpoint := strings.TrimSpace(opts.Endpoint)
	if endpoint == "" {
		apiVersion := strings.TrimSpace(opts.APIVersion)
		if apiVersion == "" {
			apiVersion = defaultAPIVersion
		}
		endpoint = uploadEndpoint(apiVersion, cfg.CustomerID)
	}

	now := opts.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}

	return &Client{cfg: cfg, endpoint: endpoint, httpClient: httpClient, tokenSource: tokenSource, now: now}, nil
}

// Validate returns an error when required Google Ads config fields are missing.
func (cfg Config) Validate() error {
	missing := make([]string, 0, 5)
	fields := []struct {
		name  string
		value string
	}{
		{name: "CustomerID", value: CleanCustomerID(cfg.CustomerID)},
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
		return fmt.Errorf("google ads configuration is missing %s", strings.Join(missing, ", "))
	}

	return nil
}

// CleanCustomerID removes Google Ads customer ID formatting dashes and whitespace.
func CleanCustomerID(raw string) string {
	return strings.ReplaceAll(strings.TrimSpace(raw), "-", "")
}

// HashEmail normalizes and hashes an email address for Google Ads enhanced conversions.
func HashEmail(email string) string {
	normalized := normalizeEmail(email)
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

func normalizeEmail(email string) string {
	normalized := strings.Join(strings.Fields(strings.ToLower(email)), "")
	local, domain, ok := strings.Cut(normalized, "@")
	if !ok {
		return normalized
	}

	switch domain {
	case "gmail.com", "googlemail.com":
		local = strings.ReplaceAll(local, ".", "")
	}

	return local + "@" + domain
}

// UploadLeadConversion uploads one lead conversion to Google Ads.
func (c *Client) UploadLeadConversion(ctx context.Context, params LeadConversionParams) error {
	return c.uploadLeadConversion(ctx, params, false)
}

// ValidateLeadConversion validates one lead conversion with Google Ads without uploading it.
func (c *Client) ValidateLeadConversion(ctx context.Context, params LeadConversionParams) error {
	return c.uploadLeadConversion(ctx, params, true)
}

func (c *Client) uploadLeadConversion(ctx context.Context, params LeadConversionParams, validateOnly bool) error {
	email := strings.TrimSpace(params.Email)
	if email == "" {
		return fmt.Errorf("google ads lead conversion email is required")
	}
	conversionActionID := strings.TrimSpace(params.ConversionActionID)
	if conversionActionID == "" {
		return fmt.Errorf("google ads lead conversion conversion action id is required")
	}

	clickID, err := clickIDFromLeadConversionParams(params)
	if err != nil {
		return err
	}

	conversionTime := params.ConversionTime
	if conversionTime.IsZero() {
		conversionTime = c.now()
	}
	conversionTime = conversionTime.UTC()

	conversion := clickConversion{
		ConversionAction:   fmt.Sprintf("customers/%s/conversionActions/%s", c.cfg.CustomerID, conversionActionID),
		ConversionDateTime: conversionTime.Format(conversionTimeLayout),
		ConversionValue:    params.ConversionValue,
		CurrencyCode:       strings.TrimSpace(params.CurrencyCode),
		GCLID:              clickID.GCLID,
		GBRAID:             clickID.GBRAID,
		WBRAID:             clickID.WBRAID,
		UserIdentifiers: []UserIdentifier{
			{HashedEmail: HashEmail(email)},
		},
	}
	payload := conversionPayload{
		PartialFailure: true,
		ValidateOnly:   validateOnly,
		Conversions:    []clickConversion{conversion},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal google ads conversion: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create google ads conversion request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("developer-token", c.cfg.DeveloperToken)
	if c.cfg.LoginCustomerID != "" {
		req.Header.Set("login-customer-id", c.cfg.LoginCustomerID)
	}
	if authErr := c.authorize(req); authErr != nil {
		return authErr
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("upload google ads conversion: %w", err)
	}
	defer resp.Body.Close()

	respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, responseMaxSize))
	if readErr != nil {
		return fmt.Errorf("read google ads conversion response: %w", readErr)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode > 299 {
		return googleAPIError(resp.StatusCode, respBody)
	}

	var parsed uploadResponse
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &parsed); err != nil {
			return fmt.Errorf("parse google ads conversion response: %w", err)
		}
	}
	if parsed.PartialFailureError.Message != "" {
		return PartialFailureError{Message: parsed.PartialFailureError.Message}
	}

	return nil
}

func (c *Client) authorize(req *http.Request) error {
	token, err := c.tokenSource.Token(req.Context())
	if err != nil {
		return fmt.Errorf("get google ads token: %w", err)
	}
	token.SetAuthHeader(req)
	return nil
}

func (s *refreshTokenSource) Token(ctx context.Context) (*oauth2.Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.token != nil && s.token.Valid() {
		return s.token, nil
	}

	refreshToken := s.cfg.OAuthRefreshToken
	if s.token != nil && s.token.RefreshToken != "" {
		refreshToken = s.token.RefreshToken
	}
	tokenCtx := context.WithValue(ctx, oauth2.HTTPClient, s.httpClient)
	token, err := oauthConfig(s.cfg).TokenSource(tokenCtx, &oauth2.Token{RefreshToken: refreshToken}).Token()
	if err != nil {
		return nil, err
	}

	s.token = token
	return token, nil
}

func oauthConfig(cfg Config) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     cfg.OAuthClientID,
		ClientSecret: cfg.OAuthClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:   googleAuthURL,
			TokenURL:  googleTokenURL,
			AuthStyle: oauth2.AuthStyleInParams,
		},
		Scopes: []string{adwordsScope},
	}
}

func clickIDFromLeadConversionParams(params LeadConversionParams) (ClickID, error) {
	gclid, err := normalizeLeadConversionClickID("gclid", params.GCLID)
	if err != nil {
		return ClickID{}, err
	}
	gbraid, err := normalizeLeadConversionClickID("gbraid", params.GBRAID)
	if err != nil {
		return ClickID{}, err
	}
	wbraid, err := normalizeLeadConversionClickID("wbraid", params.WBRAID)
	if err != nil {
		return ClickID{}, err
	}

	clickID := ClickID{
		GCLID:  gclid,
		GBRAID: gbraid,
		WBRAID: wbraid,
	}
	count := clickID.Count()
	if count != 1 {
		return ClickID{}, fmt.Errorf(requiredClickIDMessage, count)
	}

	return clickID, nil
}

func normalizeLeadConversionClickID(name, raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}

	normalized := normalizeClickID(trimmed)
	if normalized == "" {
		return "", fmt.Errorf("google ads lead conversion %s is invalid", name)
	}

	return normalized, nil
}

func uploadEndpoint(apiVersion, customerID string) string {
	return fmt.Sprintf("%s/%s/customers/%s:uploadClickConversions", googleAdsAPIBaseURL, apiVersion, customerID)
}

func googleAPIError(statusCode int, body []byte) error {
	parsed := struct {
		Error struct {
			Message string `json:"message"`
			Errors  []struct {
				Reason string `json:"reason"`
			} `json:"errors"`
		} `json:"error"`
	}{}
	message := http.StatusText(statusCode)
	if len(body) > 0 {
		if err := json.Unmarshal(body, &parsed); err != nil {
			bodyMessage := strings.TrimSpace(string(body))
			if bodyMessage != "" {
				message = bodyMessage
			}
			return &googleapi.Error{Code: statusCode, Message: message, Body: string(body)}
		}
	}
	if parsed.Error.Message != "" {
		message = parsed.Error.Message
	}

	apiErr := &googleapi.Error{
		Code:    statusCode,
		Message: message,
		Body:    string(body),
	}
	if len(parsed.Error.Errors) > 0 {
		apiErr.Errors = []googleapi.ErrorItem{{Reason: parsed.Error.Errors[0].Reason}}
	}

	return apiErr
}
