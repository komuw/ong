// Package octx houses context keys used by multiple ong packages.
package octx

type (
	logContextKeyType  string
	fingerPrintKeyType string
)

const (
	// LogCtxKey is the name of the context key used to store the logID.
	// It is used primarily by `ong/log`, `ong/client` and `ong/middleware` packages
	LogCtxKey = logContextKeyType("Ong-logID")

	// FingerPrintCtxKey is the name of the context key used to store the TLS fingerprint.
	FingerPrintCtxKey = fingerPrintKeyType("fingerPrintKeyType")
)
