package bindings

import (
	"testing"

	"github.com/incurl/bitmaher/internal/downloads"
)

// TestTheSavedCredentialIsUsedByADownload verifies that a credential
// the user saved is used by the downloads of that site.
//
// The database is the record of truth: the user saved the credential
// once, and every download of that site finds it without them typing
// it again.
func TestTheSavedCredentialIsUsedByADownload(t *testing.T) {
	service := newTestService(t)

	if err := service.HTTPAuthSave("https://example.com/file.zip", "peter", "hunter2"); err != nil {
		t.Fatalf("save the credential: %v", err)
	}

	request := downloads.AddRequest{
		Kind: downloads.KindHTTP,
		URI:  "https://example.com/file.zip",
	}
	service.applySavedCredential(&request)

	if request.Options["http-user"] != "peter" {
		t.Errorf("the user name is %q, want %q", request.Options["http-user"], "peter")
	}
	if request.Options["http-passwd"] != "hunter2" {
		t.Errorf("the password is %q, want %q", request.Options["http-passwd"], "hunter2")
	}
}

// TestTheCredentialTheUserTypedWins verifies that what the user typed
// is what the download uses.
//
// The saved credential is the fallback, not the override.
func TestTheCredentialTheUserTypedWins(t *testing.T) {
	service := newTestService(t)

	if err := service.HTTPAuthSave("https://example.com/file.zip", "peter", "hunter2"); err != nil {
		t.Fatalf("save the credential: %v", err)
	}

	request := downloads.AddRequest{
		Kind: downloads.KindHTTP,
		URI:  "https://example.com/file.zip",
		Options: map[string]string{
			"http-user":   "someone-else",
			"http-passwd": "typed-now",
		},
	}
	service.applySavedCredential(&request)

	if request.Options["http-user"] != "someone-else" {
		t.Errorf("the user name is %q, want %q", request.Options["http-user"], "someone-else")
	}
	if request.Options["http-passwd"] != "typed-now" {
		t.Errorf("the password is %q, want %q", request.Options["http-passwd"], "typed-now")
	}
}

// TestTheCredentialOfAnotherSiteIsNotUsed verifies that the credential
// of one site does not go with a download of another.
func TestTheCredentialOfAnotherSiteIsNotUsed(t *testing.T) {
	service := newTestService(t)

	if err := service.HTTPAuthSave("https://example.com/file.zip", "peter", "hunter2"); err != nil {
		t.Fatalf("save the credential: %v", err)
	}

	request := downloads.AddRequest{
		Kind: downloads.KindHTTP,
		URI:  "https://other.example.com/file.zip",
	}
	service.applySavedCredential(&request)

	if request.Options["http-user"] != "" {
		t.Errorf("the user name is %q, want none", request.Options["http-user"])
	}
}

// TestTheSavedCredentialIsFoundByTheBackend verifies the round trip of
// the credential through the bound methods.
//
// The frontend saves it, and the frontend reads it back: the database
// is the record of truth, and both go through it.
func TestTheSavedCredentialIsFoundByTheBackend(t *testing.T) {
	service := newTestService(t)

	if err := service.HTTPAuthSave("https://example.com", "peter", "hunter2"); err != nil {
		t.Fatalf("save the credential: %v", err)
	}

	credential, err := service.HTTPAuthFind("https://example.com")
	if err != nil {
		t.Fatalf("find the credential: %v", err)
	}
	if credential == nil {
		t.Fatal("the credential was not found")
	}
	if credential.Username != "peter" {
		t.Errorf("the user name is %q, want %q", credential.Username, "peter")
	}
	if credential.Password != "hunter2" {
		t.Errorf("the password is %q, want %q", credential.Password, "hunter2")
	}

	// The moment of the use is recorded, and it survives the read.
	if err := service.HTTPAuthMarkUsed(credential.ID); err != nil {
		t.Fatalf("mark the credential as used: %v", err)
	}

	credential, err = service.HTTPAuthFind("https://example.com")
	if err != nil {
		t.Fatalf("find the credential: %v", err)
	}
	if credential == nil {
		t.Fatal("the credential was not found")
	}
	if credential.LastUsedAt == "" {
		t.Error("the moment of the use was not recorded")
	}

	// An origin that has no credential is nothing, and not a record
	// with empty fields.
	missing, err := service.HTTPAuthFind("https://other.example.com")
	if err != nil {
		t.Fatalf("find the credential: %v", err)
	}
	if missing != nil {
		t.Errorf("the credential of an unknown origin is %v, want nothing", missing)
	}
}
