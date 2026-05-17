# CAPI

CAPI is a Go SDK for implementing server-side conversion APIs across advertising and analytics platforms.

The goal is to provide a consistent, typed interface for sending conversion events from your server to supported advertising platforms.

## Status

This project is in early development. Google Ads is the only supported integration today. APIs and package structure may change as additional platform integrations are added.

## Supported Integrations

- Google Ads Enhanced Conversions

## Google Ads

> [!WARNING]
> New Google Ads developer tokens are granted test-account access only by default. After you create a Manager Account and get the developer token, request Basic Access in API Center. Google Ads performs a manual review before Basic Access is approved; expect this review to take about 3 days.
>
> If you do not specifically need server-side conversion uploads, Google's client-side website conversion tracking may be easier to set up. See Google's official setup docs: https://support.google.com/google-ads/answer/16560108.

Official Google Ads REST API documentation: https://developers.google.com/google-ads/api/rest/overview

Create a client with explicit configuration. The SDK does not read environment variables or require a specific session implementation.

```go
client, err := googleads.NewClient(ctx, googleads.Config{
	CustomerID:        "123-456-7890",
	DeveloperToken:    developerToken,
	OAuthClientID:     oauthClientID,
	OAuthClientSecret: oauthClientSecret,
	OAuthRefreshToken: oauthRefreshToken,
}, googleads.ClientOptions{})
if err != nil {
	return err
}

err = client.UploadLeadConversion(ctx, googleads.LeadConversionParams{
	Email:              "lead@example.com",
	ConversionActionID: "987654321",
	GCLID:              savedGCLID,
})
```

### Manual Google Ads credential setup

(Warning: The following may change at any time as Google makes updates to their process or UI.)

The SDK needs a Google Ads customer ID, a conversion action ID, a developer token, and OAuth refresh-token credentials. Use the same Google account for the OAuth consent flow that has access to the Google Ads account you want to upload into.

1. Enable auto-tagging in Google Ads.

   In Google Ads, open Admin or Settings, then Account settings, then Auto-tagging. Enable the checkbox and save. This makes Google append `gclid` values to ad landing-page URLs.

2. Create a server-side conversion action.

   In Google Ads, open Goals, then Conversions, then Summary. Click `+ New conversion action`, select Import, choose CRMs, files, or other data sources, select Track conversions from clicks, then click Continue. Configure the action, click Create and continue, then Done.

   Use Import from clicks. A standard website pixel conversion action is not valid for server-side `UploadClickConversions` requests.

3. Find `GOOGLE_ADS_CUSTOMER_ID`.

   In Google Ads, open the account that owns the conversion action. Copy the 10-digit customer ID from the account picker or top navigation. Dashes are optional, so `123-456-7890` and `1234567890` are equivalent.

4. Find `GOOGLE_ADS_CONVERSION_ACTION_ID`.

   In Google Ads, open Goals, then Conversions, then Summary. Click the conversion action name. In the browser URL, copy the `ctId` query parameter. For `https://ads.google.com/aw/conversions/detail?ocid=123&ctId=584739201`, the conversion action ID is `584739201`.

5. Find `GOOGLE_ADS_DEVELOPER_TOKEN`.

    In the Google Ads manager account, open Admin, then API Center, or go directly to https://ads.google.com/aw/apicenter. Copy the Developer token. OAuth credentials alone are not enough for Google Ads API uploads. The developer token must be allowed to access the account type you upload into; a token limited to test accounts cannot upload production conversions.

    Google Ads only exposes API Center and developer-token generation from Manager Accounts, historically called MCC accounts. Standard individual accounts cannot create a developer token, even when your user is the account owner or super admin.

    If you do not already have a Manager Account, create one first:

    1. Open https://ads.google.com/home/tools/manager-accounts/.
    2. Click Create a manager account.
    3. Give it a name, such as `My Company - Admin Manager`.
    4. For account type, select Manage other people's accounts, even if you only manage your own account.
    5. Submit and click Explore your account.

    Link your existing production account to the new Manager Account:

    1. Copy the 10-digit customer ID of your original production account.
    2. In the Manager Account dashboard, open Accounts from the left-hand menu.
    3. Click the blue `+` button and select Link existing account.
    4. Paste the 10-digit customer ID and send the request.
    5. Approve the request from your original account under Admin, then Access and Security, then Managers, or from the approval email.

    After the accounts are linked, switch into the Manager Account, open Admin, then API Center, or go directly to https://ads.google.com/aw/apicenter. Fill out the developer profile details and copy the issued 22-character `GOOGLE_ADS_DEVELOPER_TOKEN`. Your server can use that token to upload conversions into the linked production account.

6. Create a Google Cloud OAuth client.

   Open Google Cloud Console at https://console.cloud.google.com/. Select or create the project that will own the OAuth client. Open APIs & Services, then Library, and enable the Google Ads API, `googleads.googleapis.com`.

   Open APIs & Services, then OAuth consent screen. Configure the consent screen. If the app is external and in testing mode, add the Google user who will authorize Google Ads access as a test user.

   Open APIs & Services, then Credentials. Click Create credentials, then OAuth client ID. Choose Web application. Add this Authorized redirect URI exactly: `https://developers.google.com/oauthplayground`. Create the client.

   Copy the Client ID as `GOOGLE_ADS_OAUTH_CLIENT_ID`. Copy the Client secret as `GOOGLE_ADS_OAUTH_CLIENT_SECRET`.

7. Generate `GOOGLE_ADS_REFRESH_TOKEN`.

   Open https://developers.google.com/oauthplayground/. Click the gear icon. Enable Use your own OAuth credentials. Enter `GOOGLE_ADS_OAUTH_CLIENT_ID` and `GOOGLE_ADS_OAUTH_CLIENT_SECRET`.

   In Step 1, enter this scope: `https://www.googleapis.com/auth/adwords`. Click Authorize APIs. Sign in as a Google user with access to the target Google Ads account. Approve the consent screen.

   In Step 2, click Exchange authorization code for tokens. Copy the returned Refresh token as `GOOGLE_ADS_REFRESH_TOKEN`. Store it as a secret and do not commit it.

   If no refresh token is returned, revoke the existing grant for that OAuth client at https://myaccount.google.com/permissions, then repeat the Playground flow.

8. Set `GOOGLE_ADS_LOGIN_CUSTOMER_ID` when using a manager account.

   If the OAuth user belongs to a manager account and uploads into a linked client account, set the login customer ID to the manager account customer ID. Leave it empty when the OAuth user has direct access to `GOOGLE_ADS_CUSTOMER_ID`. Missing manager-account routing commonly causes `USER_PERMISSION_DENIED` errors.

Optional conversion value and currency are only sent when provided:

```go
err = client.UploadLeadConversion(ctx, googleads.LeadConversionParams{
	Email:              "lead@example.com",
	ConversionActionID: "123456789",
	GCLID:              savedGCLID,
	ConversionValue:    new(25.0),
	CurrencyCode:       "USD",
})
```

Save Google click IDs by adapting your session library to `googleads.SessionStore`:

```go
type MySessionStore struct{}

func (MySessionStore) GetString(r *http.Request, key string) (string, error) {
	// Load your session and return key.
	return "", nil
}

func (MySessionStore) SetString(r *http.Request, w http.ResponseWriter, key, value string) error {
	// Load your session, set key to value, and save it.
	return nil
}

clickSession, err := googleads.NewClickIDSession(MySessionStore{}, googleads.ClickIDSessionOptions{})
if err != nil {
	return err
}

if err := clickSession.Capture(r, w); err != nil {
	return err
}

clickID, err := clickSession.Get(r)
if err != nil {
	return err
}

params := googleads.LeadConversionParams{Email: "lead@example.com", ConversionActionID: "987654321"}
clickID.ApplyTo(&params)
```

Validate an end-to-end Google Ads conversion request without uploading it:

```bash
go run ./cmd/capi googleads validate \
  --customer-id 123-456-7890 \
  --conversion-action-id 987654321 \
  --developer-token "$GOOGLE_ADS_DEVELOPER_TOKEN" \
  --oauth-client-id "$GOOGLE_ADS_OAUTH_CLIENT_ID" \
  --oauth-client-secret "$GOOGLE_ADS_OAUTH_CLIENT_SECRET" \
  --oauth-refresh-token "$GOOGLE_ADS_REFRESH_TOKEN" \
  --email lead@example.com \
  --gclid test-gclid_123
```

List accessible Google Ads customers and conversion actions to find the IDs needed by the SDK:

```bash
go run ./cmd/capi googleads info \
  --developer-token "$GOOGLE_ADS_DEVELOPER_TOKEN" \
  --oauth-client-id "$GOOGLE_ADS_OAUTH_CLIENT_ID" \
  --oauth-client-secret "$GOOGLE_ADS_OAUTH_CLIENT_SECRET" \
  --oauth-refresh-token "$GOOGLE_ADS_REFRESH_TOKEN" \
  --customer-id 123-456-7890
```

Add `--login-customer-id "$GOOGLE_ADS_LOGIN_CUSTOMER_ID"` to either command when the OAuth user accesses the target account through a manager account.

## Planned Integrations

- Meta Conversions API
- Reddit Conversions API
- LinkedIn Conversions API
- TikTok Events API
- Additional server-side conversion and event APIs

## Goals

- Normalize common conversion event fields across platforms
- Preserve access to platform-specific options when needed
- Provide reliable request signing, hashing, validation, and retry behavior
- Make server-side conversion tracking easier to test and maintain

## Installation

```bash
go get github.com/davidnix/capi
```

## License

License information has not been added yet.
