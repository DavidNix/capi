# CAPI

CAPI is a Go SDK for implementing server-side conversion APIs across advertising and analytics platforms.

The goal is to provide a consistent, typed interface for sending conversion events from your server to supported advertising platforms.

## Status

This project is in early development. Google Ads is the only supported integration today. APIs and package structure may change as additional platform integrations are added.

## Supported Integrations

- Google Ads Enhanced Conversions

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
