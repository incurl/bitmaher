package bindings

import (
	"github.com/incurl/bitmaher/internal/pocketbase"
)

// The saved HTTP credentials.
//
// The credentials are the ones a site asks for: the user name and the
// password that go with a URL. They are used for the downloads that
// need them, and they are saved so that the user does not have to type
// them again.
//
// The password is encrypted at rest, with the key of the application.
// It is returned in the clear to the download that has to authenticate
// itself, and it is never written anywhere else.

// HTTPAuthCredentialView is one saved credential, as the frontend sees
// it.
type HTTPAuthCredentialView struct {
	// ID is the identifier of the credential. It is what the
	// frontend passes when it records that the credential was used.
	ID string `json:"id"`
	// Origin is the URL the credential belongs to.
	Origin string `json:"origin"`
	// Username is the user name.
	Username string `json:"username"`
	// Password is the password.
	Password string `json:"password"`
	// LastUsedAt is when the credential was last used. It is empty
	// when it was never used.
	LastUsedAt string `json:"last_used_at"`
}

// HTTPAuthSave stores a credential.
//
// The credentials of an origin are replaced: there is one credential
// per origin.
func (s *AppService) HTTPAuthSave(origin, username, password string) error {
	if s == nil || s.httpAuth == nil {
		return nil
	}
	return s.httpAuth.Save(origin, username, password)
}

// HTTPAuthFind returns the credential of an origin.
//
// The answer is a **pointer**, and not a value with a flag beside it:
// a method that returns more than one value is answered with an array,
// and the frontend has to take the first element out of it. An origin
// that has no credential is a nil pointer, which is the nothing the
// frontend expects.
func (s *AppService) HTTPAuthFind(origin string) (*HTTPAuthCredentialView, error) {
	if s == nil || s.httpAuth == nil {
		return nil, nil
	}

	credential, found, err := s.httpAuth.Find(origin)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}

	view := toHTTPAuthView(credential)
	return &view, nil
}

// HTTPAuthMarkUsed records that a credential was used.
//
// The moment is what the panel shows. It is not a secret.
func (s *AppService) HTTPAuthMarkUsed(id string) error {
	if s == nil || s.httpAuth == nil {
		return nil
	}
	return s.httpAuth.MarkUsed(id)
}

// toHTTPAuthView turns a credential into the view the frontend reads.
func toHTTPAuthView(credential pocketbase.HTTPAuthCredential) HTTPAuthCredentialView {
	return HTTPAuthCredentialView{
		ID:         credential.ID,
		Origin:     credential.Origin,
		Username:   credential.Username,
		Password:   credential.Password,
		LastUsedAt: credential.LastUsedAt,
	}
}
