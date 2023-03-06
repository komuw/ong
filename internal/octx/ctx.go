package octx

type logContextKeyType string

// LogCtxKey is the name of the context key used to store the logID.
// It is used primarily by `ong/log`, `ong/client` and `ong/middleware` packages
const LogCtxKey = logContextKeyType("Ong-logID")
