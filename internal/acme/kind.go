package acme

import "fmt"

// To help clients configure themselves with the right URLs for each ACME operation, ACME servers provide a directory object.
// Clients access the directory by sending a GET request to the directory URL.
// https://datatracker.ietf.org/doc/html/rfc8555#section-7.1.1
type directory struct {
	NewNonceURL   string `json:"newNonce,omitempty"`
	NewAccountURL string `json:"newAccount,omitempty"`
	NewOrderURL   string `json:"newOrder,omitempty"`
	NewAuthz      string `json:"newAuthz,omitempty"`
	RevokeCert    string `json:"revokeCert,omitempty"`
	KeyChange     string `json:"keyChange,omitempty"`
}

// An ACME account resource represents a set of metadata associated with an account.
// https://datatracker.ietf.org/doc/html/rfc8555#section-7.1.2
type account struct {
	Contact              []string `json:"contact,omitempty"`              // Optional
	TermsOfServiceAgreed bool     `json:"termsOfServiceAgreed,omitempty"` // Optional
	// Possible values for Status are; "valid", "deactivated", and "revoked"
	// deactivated is client initiated, while revoked is server initiated.
	Status string `json:"status,omitempty"` // Required
	Orders string `json:"orders,omitempty"` // Required
	// If OnlyReturnExisting is present with the value "true", then the server MUST NOT create a new account if one does not already exist.
	// https://datatracker.ietf.org/doc/html/rfc8555#section-7.3
	// For our usecase, we won't use it. ACME seems to return an existing account even without setting this.
	OnlyReturnExisting bool `json:"onlyReturnExisting,omitempty"` // Optional

	// This we added ourself, it is not in the object definition for directory in the rfc
	kid string `json:"-"`
}

func (a account) String() string {
	return fmt.Sprintf(`account{
  Status: %s
  Contact: %s
  TermsOfServiceAgreed: %v
  Orders: %s
  kid: %s
}`, a.Status, a.Contact, a.TermsOfServiceAgreed, a.Orders, a.kid)
}

type identifier struct {
	// Any identifier of type "dns" in a newOrder request MAY have a wildcard domain name as its value.
	// A wildcard domain name consists of a single asterisk character followed by a single full stop character ("*.")
	// followed by a domain name as defined for use in the Subject Alternate Name Extension by [RFC 5280]
	// https://datatracker.ietf.org/doc/html/rfc8555#section-7.1.3
	Type  string `json:"type,omitempty"`  // Required
	Value string `json:"value,omitempty"` // Required
}

// An ACME order object represents a client's request for a certificate and is used to track the progress of that order through to issuance.
// https://datatracker.ietf.org/doc/html/rfc8555#section-7.1.3
// The order object returned by the server represents a promise that if
// the client fulfills the server's requirements before the "expires" time, then the server
// will be willing to finalize the order upon request and issue the requested certificate.
// https://datatracker.ietf.org/doc/html/rfc8555#section-7.4
type order struct {
	Identifiers    []identifier `json:"identifiers,omitempty"`    // Required
	Authorizations []string     `json:"authorizations,omitempty"` // Required
	Status         string       `json:"status"`                   // Required
	FinalizeURL    string       `json:"finalize"`                 // Required
	CertificateURL string       `json:"certificate"`              // Required
	Expires        string       `json:"expires,omitempty"`        // Optional
	NotBefore      string       `json:"notBefore,omitempty"`      // Optional
	NotAfter       string       `json:"notAfter,omitempty"`       // Optional
	Error          acmeError    `json:"error,omitempty"`          // Optional

	// This we added ourself, it is not in the object definition for directory in the rfc
	// The client should then send a POST-as-GET request to the order resource to obtain its current state.
	// https://datatracker.ietf.org/doc/html/rfc8555#section-7.4
	OrderURL string // Optional
}

func (o order) String() string {
	return fmt.Sprintf(`order{
  Identifiers: %s
  Authorizations: %s
  Status: %s
  FinalizeURL: %s
  CertificateURL: %s
  Expires: %s
  OrderURL: %s
  Error: %v
}`, o.Identifiers, o.Authorizations, o.Status, o.FinalizeURL, o.CertificateURL, o.Expires, o.OrderURL, o.Error)
}

// https://datatracker.ietf.org/doc/html/rfc8555#section-8
type challenge struct {
	Type   string `json:"type,omitempty"` // Required
	Url    string `json:"url"`            // Required
	Status string `json:"status"`         // Required
	// https://datatracker.ietf.org/doc/html/rfc8555#section-8.3
	Token     string    `json:"token,omitempty"` // Required
	Validated string    `json:"validated"`       // Optional
	Error     acmeError `json:"error,omitempty"` // Optional
}

func (c challenge) String() string {
	return fmt.Sprintf(`challenge{
  Type: %s
  Url: %s
  Status: %s
  Token: %s
  Error: %v
}`, c.Type, c.Url, c.Status, c.Token, c.Error)
}

// https://datatracker.ietf.org/doc/html/rfc8555#section-7.1.4
type authorization struct {
	Identifier identifier  `json:"identifier,omitempty"` // Required
	Status     string      `json:"status,omitempty"`     // Required
	Challenges []challenge `json:"challenges,omitempty"` // Required
	Expires    string      `json:"expires,omitempty"`    // Optional
	Wildcard   bool        `json:"wildcard,omitempty"`   // Optional

	// This we added ourself, it is not in the object definition for authorization in the rfc
	EffectiveChallenge challenge `json:"effectiveChallenge,omitempty"`
}

func (a authorization) String() string {
	return fmt.Sprintf(`authorization{
  Identifier: %s
  Status: %s
  EffectiveChallenge: %s
  Wildcard: %v
}`, a.Identifier, a.Status, a.EffectiveChallenge, a.Wildcard)
}

// https://datatracker.ietf.org/doc/html/rfc8555#section-6.2
type protected struct {
	Alg   string `json:"alg,omitempty"`
	Nonce string `json:"nonce,omitempty"`
	Url   string `json:"url,omitempty"`

	// The "jwk" and "kid" fields are mutually exclusive. Servers MUST reject requests that contain both.
	// That mutual exclusiveness, is why this two are pointers.
	// https://datatracker.ietf.org/doc/html/rfc8555#section-6.2
	//
	// Jwk is used for newAccount & revokeCert requests.
	// Kid is used for the others
	Jwk *jwk    `json:"jwk,omitempty"`
	Kid *string `json:"kid,omitempty"`
}

// The field order of the json form of jwk is important.
// See https://tools.ietf.org/html/rfc7638#section-3.3 for details.
type jwk struct {
	Crv string `json:"crv,omitempty"`
	Kty string `json:"kty,omitempty"`
	X   string `json:"x,omitempty"`
	Y   string `json:"y,omitempty"`
}

type jsonWebSignature struct {
	Protected string `json:"protected,omitempty"`
	// Note: `Payload` should not be omitempty.
	// post-as-get requests require an empty payload.
	Payload   string `json:"payload"`
	Signature string `json:"signature,omitempty"`
}

func (j jsonWebSignature) String() string {
	return fmt.Sprintf(`jsonWebSignature{
  Protected: %s
  Payload: %s
  Signature: %s
}`, j.Protected, j.Payload, j.Signature)
}

// csr is A CSR encoding the parameters for the certificate being requested [RFC2986].
// The CSR is sent in the base64url-encoded version of the DER format.
// (Note: Because this field uses base64url, and does not include headers, it is different from PEM.)
// https://datatracker.ietf.org/doc/html/rfc8555#section-7.4
type csr struct {
	CSR string `json:"csr,omitempty"`
}

type checkStatusResp struct {
	// Both `order` and `challenge` have this field.
	// And it is the only field we care about in this func.
	Status string `json:"status,omitempty"` // Required
}

func (c checkStatusResp) String() string {
	return fmt.Sprintf(`checkStatusResp{Status: %s}`, c.Status)
}

// ACME servers can return responses with an HTTP error response code (4XX or 5XX)
// When the server responds with an error status, it SHOULD provide additional information using a problem document(defined in RFC7807)
// https://datatracker.ietf.org/doc/html/rfc8555#section-6.7
// https://datatracker.ietf.org/doc/html/rfc7807#section-3.1
type acmeError struct {
	Type string `json:"type,omitempty"`
	// Clients SHOULD display the "detail" field of all errors.
	// https://datatracker.ietf.org/doc/html/rfc8555#section-6.7
	Detail   string `json:"detail,omitempty"`
	Title    string `json:"title,omitempty"`
	Instance string `json:"instance,omitempty"`
	Status   int    `json:"-"` //  Ideally, I'd like to use `json:"status,omitempty"` , for some reason that aint working.
}

func (a acmeError) Error() string {
	return fmt.Sprintf("{type: %v, detail:%v}", a.Type, a.Detail)
}
