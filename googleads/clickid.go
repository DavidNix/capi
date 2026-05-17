package googleads

import (
	"fmt"
	"net/http"
	"strings"
)

const (
	defaultClickIDSessionKey = "google_ads_click_id"
	maxClickIDLength         = 512
)

// ClickID contains one Google Ads click identifier.
type ClickID struct {
	// GCLID is a Google click ID.
	GCLID string
	// GBRAID is a Google GBRAID click ID.
	GBRAID string
	// WBRAID is a Google WBRAID click ID.
	WBRAID string
}

// SessionStore is the minimal interface required to persist a Google Ads click ID.
type SessionStore interface {
	// GetString loads a string value from the caller's session store.
	GetString(r *http.Request, key string) (string, error)
	// SetString saves a string value into the caller's session store.
	SetString(r *http.Request, w http.ResponseWriter, key, value string) error
}

// ClickIDSessionOptions configures click ID session storage.
type ClickIDSessionOptions struct {
	// Key is the session key used to store the encoded click ID.
	Key string
}

// ClickIDSession captures and retrieves Google Ads click IDs from caller-provided sessions.
type ClickIDSession struct {
	store SessionStore
	key   string
}

// NewClickIDSession creates a session helper backed by the caller's session store.
func NewClickIDSession(store SessionStore, opts ClickIDSessionOptions) (*ClickIDSession, error) {
	if store == nil {
		return nil, fmt.Errorf("google ads click id session store is required")
	}

	key := strings.TrimSpace(opts.Key)
	if key == "" {
		key = defaultClickIDSessionKey
	}

	return &ClickIDSession{store: store, key: key}, nil
}

// Count returns the number of populated click identifier fields.
func (id ClickID) Count() int {
	count := 0
	if strings.TrimSpace(id.GCLID) != "" {
		count++
	}
	if strings.TrimSpace(id.GBRAID) != "" {
		count++
	}
	if strings.TrimSpace(id.WBRAID) != "" {
		count++
	}

	return count
}

// Empty reports whether no click identifier is set.
func (id ClickID) Empty() bool {
	return id.Count() == 0
}

// ApplyTo copies the click identifier into lead conversion params.
func (id ClickID) ApplyTo(params *LeadConversionParams) {
	params.GCLID = id.GCLID
	params.GBRAID = id.GBRAID
	params.WBRAID = id.WBRAID
}

// ClickIDFromRequest extracts a valid Google Ads click ID from request query parameters.
func ClickIDFromRequest(r *http.Request) ClickID {
	if r == nil || r.URL == nil {
		return ClickID{}
	}

	query := r.URL.Query()
	clickID := ClickID{
		GCLID:  normalizeClickID(query.Get("gclid")),
		GBRAID: normalizeClickID(query.Get("gbraid")),
		WBRAID: normalizeClickID(query.Get("wbraid")),
	}
	if clickID.Count() != 1 {
		return ClickID{}
	}

	return clickID
}

// Capture persists a valid Google Ads click ID from request query parameters into the session.
func (s *ClickIDSession) Capture(r *http.Request, w http.ResponseWriter) error {
	clickID := ClickIDFromRequest(r)
	if clickID.Empty() {
		return nil
	}

	encoded := encodeClickID(clickID)
	current, err := s.store.GetString(r, s.key)
	if err != nil {
		return fmt.Errorf("get google ads click id session: %w", err)
	}
	if current == encoded {
		return nil
	}

	if err := s.store.SetString(r, w, s.key, encoded); err != nil {
		return fmt.Errorf("save google ads click id session: %w", err)
	}

	return nil
}

// Get retrieves the saved Google Ads click ID from the session.
func (s *ClickIDSession) Get(r *http.Request) (ClickID, error) {
	encoded, err := s.store.GetString(r, s.key)
	if err != nil {
		return ClickID{}, fmt.Errorf("get google ads click id session: %w", err)
	}

	return decodeClickID(encoded), nil
}

// GetGCLID retrieves the saved Google click ID from the session.
func (s *ClickIDSession) GetGCLID(r *http.Request) (string, error) {
	clickID, err := s.Get(r)
	if err != nil {
		return "", err
	}

	return clickID.GCLID, nil
}

func encodeClickID(clickID ClickID) string {
	switch {
	case clickID.GCLID != "":
		return "gclid:" + clickID.GCLID
	case clickID.GBRAID != "":
		return "gbraid:" + clickID.GBRAID
	case clickID.WBRAID != "":
		return "wbraid:" + clickID.WBRAID
	default:
		return ""
	}
}

func decodeClickID(encoded string) ClickID {
	prefix, value, ok := strings.Cut(strings.TrimSpace(encoded), ":")
	if !ok {
		return ClickID{}
	}
	value = normalizeClickID(value)
	if value == "" {
		return ClickID{}
	}

	switch prefix {
	case "gclid":
		return ClickID{GCLID: value}
	case "gbraid":
		return ClickID{GBRAID: value}
	case "wbraid":
		return ClickID{WBRAID: value}
	default:
		return ClickID{}
	}
}

func normalizeClickID(raw string) string {
	clickID := strings.TrimSpace(raw)
	if clickID == "" || len(clickID) > maxClickIDLength {
		return ""
	}

	for _, ch := range clickID {
		switch {
		case ch >= 'a' && ch <= 'z':
			continue
		case ch >= 'A' && ch <= 'Z':
			continue
		case ch >= '0' && ch <= '9':
			continue
		case ch == '-' || ch == '_':
			continue
		default:
			return ""
		}
	}

	return clickID
}
