// Package googleads uploads server-side Google Ads click conversions.
//
// The official Google Ads REST API documentation starts at
// https://developers.google.com/google-ads/api/rest/overview.
//
// # Auto-tagging
//
// Enable auto-tagging before relying on GCLID-based attribution. Auto-tagging
// appends a gclid query parameter to ad landing-page URLs. Capture that value
// when the visitor arrives, store it in a cookie, session, or database record,
// and send it back when the lead converts.
//
// # Setup
//
// See the repository README for the complete manual Google Ads setup flow,
// including where to find the customer ID, conversion action ID, developer
// token, OAuth client credentials, and refresh token.
//
// A server-side Google Ads conversion upload needs these target values:
//
//   - Config.CustomerID
//   - Config.DeveloperToken
//   - Config.OAuthClientID
//   - Config.OAuthClientSecret
//   - Config.OAuthRefreshToken
//   - LeadConversionParams.ConversionActionID
//
// The refresh token must be issued for a user with access to the target Google
// Ads account and the https://www.googleapis.com/auth/adwords OAuth scope.
// Client exchanges the refresh token for short-lived access tokens.
//
// Config.LoginCustomerID is conditionally required. Set it to the manager
// account customer ID when the refresh token belongs to a manager account and
// the conversion is uploaded into a linked client account. Leave it empty when
// the authenticated user has direct access to Config.CustomerID. If this
// routing ID is missing for manager-account auth, Google Ads commonly returns a
// USER_PERMISSION_DENIED error.
//
// The developer token must also be allowed to access the account type you are
// uploading into. A developer token that is limited to test accounts cannot
// upload conversions for production Google Ads accounts.
//
// # Click IDs and attribution
//
// Each uploaded conversion must identify the ad click it belongs to. Capture one
// and only one of gclid, gbraid, or wbraid from the landing-page URL when the
// visitor arrives, persist it in a cookie, session, or database record, and pass
// it into LeadConversionParams at conversion time. GCLID is the standard Google
// click ID. GBRAID and WBRAID cover app and web attribution cases where iOS
// privacy restrictions prevent normal GCLID attribution.
//
// # Enhanced conversions
//
// This package sends a hashed email address in userIdentifiers for enhanced
// conversions for leads. Provide the raw email in LeadConversionParams.Email;
// HashEmail normalizes it and applies SHA-256 before upload. Enable enhanced
// conversions for leads in the Google Ads account when you rely on user
// identifiers for better matching. In Google Ads, review the enhanced
// conversions settings, accept the required customer data terms, and choose API
// as the implementation method when prompted.
//
// Callers are responsible for collecting and honoring user consent before
// uploading first-party user identifiers. This package hashes the email before
// upload, but hashing does not remove the need to comply with Google's customer
// data policies and applicable privacy requirements. Consent handling is
// intentionally out of scope for this package: it does not model or transmit
// Google Ads consent fields. Library consumers should decide whether each
// upload is permitted before calling Client.UploadLeadConversion or
// Client.ValidateLeadConversion.
//
// # Time and value requirements
//
// Google Ads requires conversionDateTime values with an explicit UTC offset in
// the format yyyy-mm-dd hh:mm:ss+|-hh:mm. Client formats conversion times in UTC
// using that required shape. Use ConversionValue and CurrencyCode when the
// conversion action is configured to use different values for each conversion.
// The conversion time must be after the ad click and within the conversion
// action's click-through conversion window.
//
// Client.ValidateLeadConversion sends the same payload shape as
// Client.UploadLeadConversion with validateOnly enabled. Use it to confirm the
// customer ID, conversion action ID, developer token, OAuth credentials, login
// customer routing, click ID, and enhanced-conversion setup before enabling real
// uploads.
package googleads
