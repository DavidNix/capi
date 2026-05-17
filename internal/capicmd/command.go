package capicmd

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/davidnix/capi/googleads"
	"github.com/spf13/cobra"
)

const (
	requestTimeout      = 30 * time.Second
	googleAdsOAuthScope = "https://www.googleapis.com/auth/adwords"
)

type googleAdsValidateOptions struct {
	customerID         string
	conversionActionID string
	developerToken     string
	oauthClientID      string
	oauthClientSecret  string
	oauthRefreshToken  string
	loginCustomerID    string
	email              string
	gclid              string
	gbraid             string
	wbraid             string
	conversionValue    string
	currencyCode       string
	conversionTime     string
	apiVersion         string
	endpoint           string
}

type googleAdsInfoOptions struct {
	developerToken    string
	oauthClientID     string
	oauthClientSecret string
	oauthRefreshToken string
	loginCustomerID   string
	customerID        string
	apiVersion        string
	baseURL           string
}

type requiredFlag struct {
	name  string
	value string
}

// NewRootCommand creates the capi CLI root command.
func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "capi",
		Short:         "Conversion API tools",
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.NoArgs,
	}
	rootCmd.AddCommand(newGoogleAdsCommand())

	return rootCmd
}

func newGoogleAdsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "googleads",
		Short: "Google Ads conversion tools",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(newGoogleAdsInfoCommand())
	cmd.AddCommand(newGoogleAdsValidateCommand())

	return cmd
}

func newGoogleAdsInfoCommand() *cobra.Command {
	infoOpts := googleAdsInfoOptions{}
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Discover Google Ads customer and conversion action IDs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runGoogleAdsInfo(cmd, infoOpts)
		},
	}
	cmd.Flags().StringVar(&infoOpts.developerToken, "developer-token", "", "Google Ads developer token")
	cmd.Flags().StringVar(&infoOpts.oauthClientID, "oauth-client-id", "", "OAuth client ID")
	cmd.Flags().StringVar(&infoOpts.oauthClientSecret, "oauth-client-secret", "", "OAuth client secret")
	cmd.Flags().StringVar(&infoOpts.oauthRefreshToken, "oauth-refresh-token", "", "OAuth refresh token")
	cmd.Flags().StringVar(&infoOpts.loginCustomerID, "login-customer-id", "", "Optional manager account customer ID")
	cmd.Flags().StringVar(&infoOpts.customerID, "customer-id", "", "Optional Google Ads customer ID to inspect")
	cmd.Flags().StringVar(&infoOpts.apiVersion, "api-version", "", "Optional Google Ads API version, such as v24")
	cmd.Flags().StringVar(&infoOpts.baseURL, "base-url", "", "Optional Google Ads API base URL")

	return cmd
}

func newGoogleAdsValidateCommand() *cobra.Command {
	validateOpts := googleAdsValidateOptions{}
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate a Google Ads lead conversion without uploading it",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runGoogleAdsValidate(cmd, validateOpts)
		},
	}
	cmd.Flags().StringVar(&validateOpts.customerID, "customer-id", "", "Google Ads customer ID")
	cmd.Flags().StringVar(&validateOpts.conversionActionID, "conversion-action-id", "", "Google Ads conversion action ID")
	cmd.Flags().StringVar(&validateOpts.developerToken, "developer-token", "", "Google Ads developer token")
	cmd.Flags().StringVar(&validateOpts.oauthClientID, "oauth-client-id", "", "OAuth client ID")
	cmd.Flags().StringVar(&validateOpts.oauthClientSecret, "oauth-client-secret", "", "OAuth client secret")
	cmd.Flags().StringVar(&validateOpts.oauthRefreshToken, "oauth-refresh-token", "", "OAuth refresh token")
	cmd.Flags().StringVar(&validateOpts.loginCustomerID, "login-customer-id", "", "Optional manager account customer ID")
	cmd.Flags().StringVar(&validateOpts.email, "email", "", "Lead email address for enhanced conversions")
	cmd.Flags().StringVar(&validateOpts.gclid, "gclid", "", "Google click ID")
	cmd.Flags().StringVar(&validateOpts.gbraid, "gbraid", "", "Google GBRAID click ID")
	cmd.Flags().StringVar(&validateOpts.wbraid, "wbraid", "", "Google WBRAID click ID")
	cmd.Flags().StringVar(&validateOpts.conversionValue, "conversion-value", "", "Optional conversion value")
	cmd.Flags().StringVar(&validateOpts.currencyCode, "currency-code", "", "Optional ISO 4217 currency code")
	cmd.Flags().StringVar(&validateOpts.conversionTime, "conversion-time", "", "Optional conversion time in RFC3339 format")
	cmd.Flags().StringVar(&validateOpts.apiVersion, "api-version", "", "Optional Google Ads API version, such as v24")
	cmd.Flags().StringVar(&validateOpts.endpoint, "endpoint", "", "Optional full uploadClickConversions endpoint override")

	return cmd
}

func runGoogleAdsValidate(cmd *cobra.Command, validateOpts googleAdsValidateOptions) error {
	if err := requireFlags([]requiredFlag{
		{name: "--customer-id", value: validateOpts.customerID},
		{name: "--conversion-action-id", value: validateOpts.conversionActionID},
		{name: "--developer-token", value: validateOpts.developerToken},
		{name: "--oauth-client-id", value: validateOpts.oauthClientID},
		{name: "--oauth-client-secret", value: validateOpts.oauthClientSecret},
		{name: "--oauth-refresh-token", value: validateOpts.oauthRefreshToken},
		{name: "--email", value: validateOpts.email},
	}); err != nil {
		return err
	}

	parsedValue, err := parseOptionalFloat(validateOpts.conversionValue)
	if err != nil {
		return fmt.Errorf("invalid --conversion-value: %w", err)
	}
	parsedTime, err := parseOptionalTime(validateOpts.conversionTime)
	if err != nil {
		return fmt.Errorf("invalid --conversion-time: %w", err)
	}

	client, err := googleads.NewClient(cmd.Context(), googleads.Config{
		CustomerID:        validateOpts.customerID,
		DeveloperToken:    validateOpts.developerToken,
		OAuthClientID:     validateOpts.oauthClientID,
		OAuthClientSecret: validateOpts.oauthClientSecret,
		OAuthRefreshToken: validateOpts.oauthRefreshToken,
		LoginCustomerID:   validateOpts.loginCustomerID,
	}, googleads.ClientOptions{
		HTTPClient: &http.Client{Timeout: requestTimeout},
		Endpoint:   validateOpts.endpoint,
		APIVersion: validateOpts.apiVersion,
	})
	if err != nil {
		return fmt.Errorf("initialize google ads client: %w", err)
	}

	err = client.ValidateLeadConversion(cmd.Context(), googleads.LeadConversionParams{
		Email:              validateOpts.email,
		ConversionActionID: validateOpts.conversionActionID,
		GCLID:              validateOpts.gclid,
		GBRAID:             validateOpts.gbraid,
		WBRAID:             validateOpts.wbraid,
		ConversionTime:     parsedTime,
		ConversionValue:    parsedValue,
		CurrencyCode:       validateOpts.currencyCode,
	})
	if err != nil {
		return fmt.Errorf("validate google ads lead conversion: %w", err)
	}

	if _, err := fmt.Fprintln(cmd.OutOrStdout(), "Google Ads validate-only conversion succeeded"); err != nil {
		return fmt.Errorf("write google ads validate result: %w", err)
	}

	return nil
}

func runGoogleAdsInfo(cmd *cobra.Command, infoOpts googleAdsInfoOptions) error {
	if err := requireFlags([]requiredFlag{
		{name: "--developer-token", value: infoOpts.developerToken},
		{name: "--oauth-client-id", value: infoOpts.oauthClientID},
		{name: "--oauth-client-secret", value: infoOpts.oauthClientSecret},
		{name: "--oauth-refresh-token", value: infoOpts.oauthRefreshToken},
	}); err != nil {
		return fmt.Errorf("%w\n\nGenerate GOOGLE_ADS_REFRESH_TOKEN with the Google OAuth 2.0 Playground at https://developers.google.com/oauthplayground/ using scope %s", err, googleAdsOAuthScope)
	}

	discovery, err := googleads.NewDiscoveryClient(cmd.Context(), googleads.Config{
		CustomerID:        infoOpts.customerID,
		DeveloperToken:    infoOpts.developerToken,
		OAuthClientID:     infoOpts.oauthClientID,
		OAuthClientSecret: infoOpts.oauthClientSecret,
		OAuthRefreshToken: infoOpts.oauthRefreshToken,
		LoginCustomerID:   infoOpts.loginCustomerID,
	}, googleads.DiscoveryOptions{
		HTTPClient: &http.Client{Timeout: requestTimeout},
		BaseURL:    infoOpts.baseURL,
		APIVersion: infoOpts.apiVersion,
	})
	if err != nil {
		return fmt.Errorf("initialize google ads discovery client: %w", err)
	}

	customers, err := discovery.ListAccessibleCustomers(cmd.Context())
	if err != nil {
		return fmt.Errorf("list google ads accessible customers: %w", err)
	}
	if printErr := printAccessibleCustomers(cmd.OutOrStdout(), customers, infoOpts.customerID); printErr != nil {
		return printErr
	}

	customerID := googleads.CleanCustomerID(infoOpts.customerID)
	if customerID == "" {
		return nil
	}

	actions, err := discovery.ListConversionActions(cmd.Context(), customerID)
	if err != nil {
		return fmt.Errorf("list google ads conversion actions: %w", err)
	}
	if printErr := printConversionActions(cmd.OutOrStdout(), customerID, googleads.CleanCustomerID(infoOpts.loginCustomerID), actions); printErr != nil {
		return printErr
	}

	return nil
}

func printAccessibleCustomers(stdout io.Writer, customers []googleads.AccessibleCustomer, selectedCustomerID string) error {
	if _, err := fmt.Fprintln(stdout, "Google Ads accessible customers:"); err != nil {
		return fmt.Errorf("write google ads accessible customers heading: %w", err)
	}
	if len(customers) == 0 {
		if _, err := fmt.Fprintln(stdout, "none"); err != nil {
			return fmt.Errorf("write google ads accessible customers empty result: %w", err)
		}
		return nil
	}

	for _, customer := range customers {
		if _, err := fmt.Fprintf(stdout, "- %s (%s)\n", customer.CustomerID, customer.ResourceName); err != nil {
			return fmt.Errorf("write google ads accessible customer: %w", err)
		}
	}

	if googleads.CleanCustomerID(selectedCustomerID) != "" {
		return nil
	}
	if _, err := fmt.Fprintf(stdout, "\nRun again with --customer-id %s to inspect conversion actions.\n", customers[0].CustomerID); err != nil {
		return fmt.Errorf("write google ads customer id hint: %w", err)
	}
	if _, err := fmt.Fprintf(stdout, "\nexport GOOGLE_ADS_CUSTOMER_ID=%s\n", customers[0].CustomerID); err != nil {
		return fmt.Errorf("write google ads customer id export: %w", err)
	}

	return nil
}

func printConversionActions(stdout io.Writer, customerID, loginCustomerID string, actions []googleads.ConversionAction) error {
	if _, err := fmt.Fprintf(stdout, "\nGoogle Ads conversion actions for %s:\n", customerID); err != nil {
		return fmt.Errorf("write google ads conversion actions heading: %w", err)
	}
	if len(actions) == 0 {
		if _, err := fmt.Fprintln(stdout, "none"); err != nil {
			return fmt.Errorf("write google ads conversion actions empty result: %w", err)
		}
		return nil
	}

	var firstUploadAction *googleads.ConversionAction
	for i := range actions {
		action := actions[i]
		valid := "no"
		if action.SupportsClickConversions() {
			valid = "yes"
			if action.Status == "ENABLED" && firstUploadAction == nil {
				firstUploadAction = &actions[i]
			}
		}
		if _, err := fmt.Fprintf(stdout, "- %s %q status=%s type=%s category=%s upload_clicks=%s\n", action.ID, action.Name, action.Status, action.Type, action.Category, valid); err != nil {
			return fmt.Errorf("write google ads conversion action: %w", err)
		}
	}

	if firstUploadAction == nil {
		if _, err := fmt.Fprintln(stdout, "\nNo enabled conversion actions with type UPLOAD_CLICKS found. Create an Import from clicks action before uploading server-side conversions."); err != nil {
			return fmt.Errorf("write google ads conversion action warning: %w", err)
		}
		return nil
	}

	if _, err := fmt.Fprintln(stdout, "\nCandidate environment values:"); err != nil {
		return fmt.Errorf("write google ads env heading: %w", err)
	}
	if _, err := fmt.Fprintf(stdout, "export GOOGLE_ADS_CUSTOMER_ID=%s\n", customerID); err != nil {
		return fmt.Errorf("write google ads customer id export: %w", err)
	}
	if loginCustomerID != "" {
		if _, err := fmt.Fprintf(stdout, "export GOOGLE_ADS_LOGIN_CUSTOMER_ID=%s\n", loginCustomerID); err != nil {
			return fmt.Errorf("write google ads login customer id export: %w", err)
		}
	}
	if _, err := fmt.Fprintf(stdout, "export GOOGLE_ADS_CONVERSION_ACTION_ID=%s\n", firstUploadAction.ID); err != nil {
		return fmt.Errorf("write google ads conversion action id export: %w", err)
	}

	return nil
}

func requireFlags(values []requiredFlag) error {
	missing := make([]string, 0, len(values))
	for _, required := range values {
		if strings.TrimSpace(required.value) == "" {
			missing = append(missing, required.name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required flags: %s", strings.Join(missing, ", "))
	}

	return nil
}

func parseOptionalFloat(raw string) (*float64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}

	value, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return nil, err
	}

	return &value, nil
}

func parseOptionalTime(raw string) (time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}, nil
	}

	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return time.Time{}, err
	}

	return parsed.UTC(), nil
}
