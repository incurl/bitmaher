package pocketbase

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// CollectionHTTPAuth holds the saved HTTP credentials.
//
// The credentials are the ones a site asks for: the user name and the
// password that go with a URL. They are used for the downloads that
// need them, and they are saved so that the user does not have to type
// them again.
//
// The collection is closed for reading as well as for writing: the
// credentials are the backend's business, and the frontend asks for the
// one that matches a URL.
const CollectionHTTPAuth = "http_auth"

// HTTPAuthCredential is one saved credential.
//
// The password is returned in the clear, because the caller is the
// download that has to authenticate itself. It never leaves the
// process.
type HTTPAuthCredential struct {
	// ID is the identifier of the record.
	ID string `json:"id"`
	// Origin is the URL the credential belongs to.
	Origin string `json:"origin"`
	// Username is the user name.
	Username string `json:"username"`
	// Password is the password, in the clear.
	Password string `json:"password"`
	// LastUsedAt is when the credential was last used. It is empty
	// when it was never used.
	LastUsedAt string `json:"last_used_at"`
}

// HTTPAuthStore keeps the saved HTTP credentials.
//
// It is the only place that knows how they are stored: the password is
// encrypted with the key of the application, and it is matched by the
// origin of the URL.
type HTTPAuthStore struct {
	app    core.App
	cipher *Cipher
}

// NewHTTPAuthStore creates a store for the saved HTTP credentials.
func NewHTTPAuthStore(app core.App, cipher *Cipher) *HTTPAuthStore {
	return &HTTPAuthStore{app: app, cipher: cipher}
}

// Save stores a credential.
//
// The credentials of an origin are replaced: there is one credential
// per origin, and a new one for the same origin is the old one with a
// different password.
func (s *HTTPAuthStore) Save(origin, username, password string) error {
	if s == nil || s.app == nil {
		return fmt.Errorf("pocketbase: the http auth store is not available")
	}

	origin = normaliseOrigin(origin)
	if origin == "" {
		return fmt.Errorf("pocketbase: the origin of the credential is missing")
	}

	envelope, err := s.cipher.Encrypt([]byte(password))
	if err != nil {
		return fmt.Errorf("pocketbase: encrypt the credential: %w", err)
	}

	record, err := s.findByOrigin(origin)
	if err != nil {
		return err
	}
	if record == nil {
		record = core.NewRecord(s.collection())
		record.Set("origin", origin)
	}

	record.Set("username", username)
	record.Set("password", envelope)

	if err := s.app.Save(record); err != nil {
		return fmt.Errorf("pocketbase: save the credential: %w", err)
	}
	return nil
}

// Find returns the credential of an origin.
//
// The origin is matched exactly, and then by its host: a credential
// saved for "https://example.com" is used for
// "https://example.com/path".
func (s *HTTPAuthStore) Find(origin string) (HTTPAuthCredential, bool, error) {
	if s == nil || s.app == nil {
		return HTTPAuthCredential{}, false, nil
	}

	origin = normaliseOrigin(origin)
	if origin == "" {
		return HTTPAuthCredential{}, false, nil
	}

	record, err := s.findByOrigin(origin)
	if err != nil {
		return HTTPAuthCredential{}, false, err
	}
	if record == nil {
		return HTTPAuthCredential{}, false, nil
	}

	return s.toCredential(record)
}

// MarkUsed records that a credential was used.
func (s *HTTPAuthStore) MarkUsed(id string) error {
	if s == nil || s.app == nil {
		return nil
	}

	record, err := s.app.FindRecordById(CollectionHTTPAuth, id)
	if err != nil {
		// A credential that is not there is not a failure: it was
		// removed between the read and the write.
		return nil
	}

	record.Set("last_used_at", time.Now().UTC().Format(time.RFC3339))
	if err := s.app.Save(record); err != nil {
		return fmt.Errorf("pocketbase: mark the credential as used: %w", err)
	}
	return nil
}

// collection returns the collection of the credentials.
func (s *HTTPAuthStore) collection() *core.Collection {
	col, err := s.app.FindCollectionByNameOrId(CollectionHTTPAuth)
	if err != nil {
		return nil
	}
	return col
}

// findByOrigin looks a credential up by its origin.
func (s *HTTPAuthStore) findByOrigin(origin string) (*core.Record, error) {
	record, err := s.app.FindFirstRecordByFilter(
		CollectionHTTPAuth,
		"origin = {:origin}",
		map[string]any{"origin": origin},
	)
	if err != nil {
		// The credential is not there: it may be about to be
		// created.
		return nil, nil
	}
	return record, nil
}

// toCredential turns a record into a credential.
//
// The password is decrypted here, and it is in the clear in the answer:
// the caller is the download that has to authenticate itself.
func (s *HTTPAuthStore) toCredential(record *core.Record) (HTTPAuthCredential, bool, error) {
	envelope := record.GetString("password")
	password := ""
	if envelope != "" {
		raw, err := s.cipher.Decrypt(envelope)
		if err != nil {
			return HTTPAuthCredential{}, false, fmt.Errorf("pocketbase: decrypt the credential: %w", err)
		}
		password = string(raw)
	}

	lastUsed := ""
	if value := record.GetDateTime("last_used_at"); !value.Time().IsZero() {
		lastUsed = value.Time().UTC().Format(time.RFC3339)
	}

	return HTTPAuthCredential{
		ID:         record.Id,
		Origin:     record.GetString("origin"),
		Username:   record.GetString("username"),
		Password:   password,
		LastUsedAt: lastUsed,
	}, true, nil
}

// normaliseOrigin reduces a URL to the origin the credential belongs
// to.
//
// The path, the query and the fragment are dropped: a credential is
// for a site, not for a page of it. The scheme and the host are
// lowered, because they are not case-sensitive.
func normaliseOrigin(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	// A URL without a scheme is read as a host.
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}

	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return ""
	}

	scheme := strings.ToLower(parsed.Scheme)
	host := strings.ToLower(parsed.Host)

	return scheme + "://" + host
}
