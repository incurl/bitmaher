package pocketbase

import (
	"testing"
)

// newTestHTTPAuthStore creates a store for the saved HTTP credentials,
// backed by a throwaway database.
func newTestHTTPAuthStore(t *testing.T) *HTTPAuthStore {
	t.Helper()

	service := NewService(t.TempDir())
	if err := service.Bootstrap(); err != nil {
		t.Fatalf("bootstrap pocketbase: %v", err)
	}
	t.Cleanup(func() {
		_ = service.Close()
	})

	cipher, err := NewCipher(service.DataDir())
	if err != nil {
		t.Fatalf("create cipher: %v", err)
	}

	return NewHTTPAuthStore(service.App(), cipher)
}

// TestTheCredentialSurvivesAWriteAndARead verifies that a credential
// comes back as it was written.
//
// The database is the record of truth: what the user saved is what the
// next download reads.
func TestTheCredentialSurvivesAWriteAndARead(t *testing.T) {
	store := newTestHTTPAuthStore(t)

	if err := store.Save("https://example.com", "peter", "hunter2"); err != nil {
		t.Fatalf("save the credential: %v", err)
	}

	credential, found, err := store.Find("https://example.com")
	if err != nil {
		t.Fatalf("find the credential: %v", err)
	}
	if !found {
		t.Fatal("the credential was not found")
	}

	if credential.Username != "peter" {
		t.Errorf("the user name is %q, want %q", credential.Username, "peter")
	}
	if credential.Password != "hunter2" {
		t.Errorf("the password is %q, want %q", credential.Password, "hunter2")
	}
	if credential.Origin != "https://example.com" {
		t.Errorf("the origin is %q, want %q", credential.Origin, "https://example.com")
	}
	if credential.ID == "" {
		t.Error("the credential has no identifier")
	}
}

// TestThePasswordIsEncryptedAtRest verifies that the password is not
// written to the database in the clear.
//
// The database is the record of truth, and it is on the disk. A
// password that is readable there is a password that is lost.
func TestThePasswordIsEncryptedAtRest(t *testing.T) {
	service := NewService(t.TempDir())
	if err := service.Bootstrap(); err != nil {
		t.Fatalf("bootstrap pocketbase: %v", err)
	}
	t.Cleanup(func() {
		_ = service.Close()
	})

	cipher, err := NewCipher(service.DataDir())
	if err != nil {
		t.Fatalf("create cipher: %v", err)
	}
	store := NewHTTPAuthStore(service.App(), cipher)

	if err := store.Save("https://example.com", "peter", "hunter2"); err != nil {
		t.Fatalf("save the credential: %v", err)
	}

	// The record is read straight out of the collection, without the
	// store decrypting it.
	records, err := service.App().FindAllRecords(CollectionHTTPAuth)
	if err != nil {
		t.Fatalf("read the records: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("the collection holds %d records, want 1", len(records))
	}

	stored := records[0].GetString("password")
	if stored == "" {
		t.Fatal("the record holds no password")
	}
	if stored == "hunter2" {
		t.Error("the password is stored in the clear")
	}
}

// TestTheOriginIsReducedToTheSite verifies that a credential belongs
// to a site, and not to a page of it.
//
// The user saves the credential once, and every download of that site
// finds it.
func TestTheOriginIsReducedToTheSite(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"https://example.com", "https://example.com"},
		{"https://example.com/path/to/file.zip", "https://example.com"},
		{"https://example.com/path?query=1#fragment", "https://example.com"},
		{"HTTPS://EXAMPLE.COM/Path", "https://example.com"},
		{"example.com", "https://example.com"},
		{"", ""},
	}

	for _, c := range cases {
		if got := normaliseOrigin(c.raw); got != c.want {
			t.Errorf("normaliseOrigin(%q) = %q, want %q", c.raw, got, c.want)
		}
	}
}

// TestTheCredentialOfASiteIsFoundByItsPage verifies that a credential
// saved for a site is found for a page of it.
func TestTheCredentialOfASiteIsFoundByItsPage(t *testing.T) {
	store := newTestHTTPAuthStore(t)

	if err := store.Save("https://example.com", "peter", "hunter2"); err != nil {
		t.Fatalf("save the credential: %v", err)
	}

	credential, found, err := store.Find("https://example.com/downloads/file.zip")
	if err != nil {
		t.Fatalf("find the credential: %v", err)
	}
	if !found {
		t.Fatal("the credential of the site was not found for a page of it")
	}
	if credential.Username != "peter" {
		t.Errorf("the user name is %q, want %q", credential.Username, "peter")
	}
}

// TestTheCredentialOfOneSiteIsNotTheCredentialOfAnother verifies that
// the credentials do not leak from one site to another.
func TestTheCredentialOfOneSiteIsNotTheCredentialOfAnother(t *testing.T) {
	store := newTestHTTPAuthStore(t)

	if err := store.Save("https://example.com", "peter", "hunter2"); err != nil {
		t.Fatalf("save the credential: %v", err)
	}

	_, found, err := store.Find("https://other.example.com")
	if err != nil {
		t.Fatalf("find the credential: %v", err)
	}
	if found {
		t.Error("the credential of one site was found for another")
	}
}

// TestTheCredentialOfAnOriginIsReplaced verifies that there is one
// credential per origin.
//
// A new password for the same site replaces the old one, and there is
// no second record.
func TestTheCredentialOfAnOriginIsReplaced(t *testing.T) {
	store := newTestHTTPAuthStore(t)

	if err := store.Save("https://example.com", "peter", "hunter2"); err != nil {
		t.Fatalf("save the credential: %v", err)
	}
	if err := store.Save("https://example.com", "peter", "hunter3"); err != nil {
		t.Fatalf("save the credential: %v", err)
	}

	credential, found, err := store.Find("https://example.com")
	if err != nil {
		t.Fatalf("find the credential: %v", err)
	}
	if !found {
		t.Fatal("the credential was not found")
	}
	if credential.Password != "hunter3" {
		t.Errorf("the password is %q, want %q", credential.Password, "hunter3")
	}
}

// TestTheMomentOfTheUseIsRecorded verifies that the store remembers
// when a credential was last used.
func TestTheMomentOfTheUseIsRecorded(t *testing.T) {
	store := newTestHTTPAuthStore(t)

	if err := store.Save("https://example.com", "peter", "hunter2"); err != nil {
		t.Fatalf("save the credential: %v", err)
	}

	credential, found, err := store.Find("https://example.com")
	if err != nil {
		t.Fatalf("find the credential: %v", err)
	}
	if !found {
		t.Fatal("the credential was not found")
	}
	if credential.LastUsedAt != "" {
		t.Errorf("the moment of the use is %q, want none", credential.LastUsedAt)
	}

	if err := store.MarkUsed(credential.ID); err != nil {
		t.Fatalf("mark the credential as used: %v", err)
	}

	credential, found, err = store.Find("https://example.com")
	if err != nil {
		t.Fatalf("find the credential: %v", err)
	}
	if !found {
		t.Fatal("the credential was not found")
	}
	if credential.LastUsedAt == "" {
		t.Error("the moment of the use was not recorded")
	}
}

// TestTheCollectionIsClosedForReading verifies that the credentials
// cannot be read through the HTTP API.
//
// They are the user's passwords, and the API is reachable by anything
// that can reach the port.
func TestTheCollectionIsClosedForReading(t *testing.T) {
	service := NewService(t.TempDir())
	if err := service.Bootstrap(); err != nil {
		t.Fatalf("bootstrap pocketbase: %v", err)
	}
	t.Cleanup(func() {
		_ = service.Close()
	})

	collection, err := service.Collection(CollectionHTTPAuth)
	if err != nil {
		t.Fatalf("find the collection: %v", err)
	}

	if collection.ListRule != nil {
		t.Error("the list rule is set: the credentials can be listed")
	}
	if collection.ViewRule != nil {
		t.Error("the view rule is set: the credentials can be read")
	}
}
