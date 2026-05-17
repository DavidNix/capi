package googleads_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/davidnix/capi/googleads"
	"github.com/stretchr/testify/require"
)

type memorySessionStore struct {
	values   map[string]string
	getErr   error
	setErr   error
	setCalls int
}

func (s *memorySessionStore) GetString(_ *http.Request, key string) (string, error) {
	if s.getErr != nil {
		return "", s.getErr
	}
	return s.values[key], nil
}

func (s *memorySessionStore) SetString(_ *http.Request, _ http.ResponseWriter, key, value string) error {
	if s.setErr != nil {
		return s.setErr
	}
	if s.values == nil {
		s.values = map[string]string{}
	}
	s.values[key] = value
	s.setCalls++
	return nil
}

func TestClickIDFromRequest(t *testing.T) {
	t.Parallel()

	t.Run("extracts one click id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?gclid=test-gclid_123", nil)

		clickID := googleads.ClickIDFromRequest(req)

		require.Equal(t, googleads.ClickID{GCLID: "test-gclid_123"}, clickID)
	})

	t.Run("ignores invalid values", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?gclid=bad%20value", nil)

		clickID := googleads.ClickIDFromRequest(req)

		require.True(t, clickID.Empty())
	})

	t.Run("ignores overly long values", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?gclid="+strings.Repeat("a", 513), nil)

		clickID := googleads.ClickIDFromRequest(req)

		require.True(t, clickID.Empty())
	})

	t.Run("ignores ambiguous values", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/?gclid=test-gclid_123&wbraid=test-wbraid_123", nil)

		clickID := googleads.ClickIDFromRequest(req)

		require.True(t, clickID.Empty())
	})
}

func TestClickIDSession(t *testing.T) {
	t.Parallel()

	t.Run("captures and retrieves gclid", func(t *testing.T) {
		store := &memorySessionStore{values: map[string]string{"anon_id": "anon-1"}}
		session, err := googleads.NewClickIDSession(store, googleads.ClickIDSessionOptions{})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/?gclid=test-gclid_123", nil)
		rec := httptest.NewRecorder()
		err = session.Capture(req, rec)
		require.NoError(t, err)

		clickID, err := session.Get(httptest.NewRequest(http.MethodGet, "/", nil))
		require.NoError(t, err)
		require.Equal(t, googleads.ClickID{GCLID: "test-gclid_123"}, clickID)
		require.Equal(t, "anon-1", store.values["anon_id"])

		gclid, err := session.GetGCLID(httptest.NewRequest(http.MethodGet, "/", nil))
		require.NoError(t, err)
		require.Equal(t, "test-gclid_123", gclid)
	})

	t.Run("latest valid click id wins", func(t *testing.T) {
		store := &memorySessionStore{}
		session, err := googleads.NewClickIDSession(store, googleads.ClickIDSessionOptions{})
		require.NoError(t, err)

		firstReq := httptest.NewRequest(http.MethodGet, "/?gclid=first-gclid_123", nil)
		err = session.Capture(firstReq, httptest.NewRecorder())
		require.NoError(t, err)

		secondReq := httptest.NewRequest(http.MethodGet, "/?gbraid=second-gbraid_123", nil)
		err = session.Capture(secondReq, httptest.NewRecorder())
		require.NoError(t, err)

		clickID, err := session.Get(httptest.NewRequest(http.MethodGet, "/", nil))
		require.NoError(t, err)
		require.Equal(t, googleads.ClickID{GBRAID: "second-gbraid_123"}, clickID)
	})

	t.Run("ignores missing or invalid click ids", func(t *testing.T) {
		store := &memorySessionStore{}
		session, err := googleads.NewClickIDSession(store, googleads.ClickIDSessionOptions{})
		require.NoError(t, err)

		err = session.Capture(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())
		require.NoError(t, err)
		err = session.Capture(httptest.NewRequest(http.MethodGet, "/?gclid=bad%20value", nil), httptest.NewRecorder())
		require.NoError(t, err)

		require.Equal(t, 0, store.setCalls)
	})

	t.Run("does not save unchanged value", func(t *testing.T) {
		store := &memorySessionStore{}
		session, err := googleads.NewClickIDSession(store, googleads.ClickIDSessionOptions{})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/?wbraid=test-wbraid_123", nil)
		err = session.Capture(req, httptest.NewRecorder())
		require.NoError(t, err)
		err = session.Capture(req, httptest.NewRecorder())
		require.NoError(t, err)

		require.Equal(t, 1, store.setCalls)
	})

	t.Run("supports custom key", func(t *testing.T) {
		store := &memorySessionStore{}
		session, err := googleads.NewClickIDSession(store, googleads.ClickIDSessionOptions{Key: "custom_google_click_id"})
		require.NoError(t, err)

		err = session.Capture(httptest.NewRequest(http.MethodGet, "/?gclid=test-gclid_123", nil), httptest.NewRecorder())
		require.NoError(t, err)

		require.Equal(t, "gclid:test-gclid_123", store.values["custom_google_click_id"])
	})

	t.Run("returns store errors", func(t *testing.T) {
		getErr := errors.New("read failed")
		store := &memorySessionStore{getErr: getErr}
		session, err := googleads.NewClickIDSession(store, googleads.ClickIDSessionOptions{})
		require.NoError(t, err)

		err = session.Capture(httptest.NewRequest(http.MethodGet, "/?gclid=test-gclid_123", nil), httptest.NewRecorder())
		require.EqualError(t, err, "get google ads click id session: read failed")

		_, err = session.Get(httptest.NewRequest(http.MethodGet, "/", nil))
		require.EqualError(t, err, "get google ads click id session: read failed")
	})

	t.Run("requires store", func(t *testing.T) {
		_, err := googleads.NewClickIDSession(nil, googleads.ClickIDSessionOptions{})

		require.EqualError(t, err, "google ads click id session store is required")
	})
}

func TestClickID_ApplyTo(t *testing.T) {
	t.Parallel()

	params := googleads.LeadConversionParams{Email: "test@example.com"}
	clickID := googleads.ClickID{WBRAID: "test-wbraid_123"}

	clickID.ApplyTo(&params)

	require.Equal(t, "test-wbraid_123", params.WBRAID)
}
