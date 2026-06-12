// Package requestid provides request ID context propagation.
//
// This is a minimal package that holds only the context key and extraction
// functions. It exists to break circular imports between the middleware
// and logger packages — both need to access the request ID from context,
// but neither should import the other.
package requestid

import "context"

type contextKey struct{}

// WithRequestID returns a new context with the given request ID stored.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, contextKey{}, id)
}

// FromContext extracts the request ID from the context.
// Returns an empty string if no request ID is set.
func FromContext(ctx context.Context) string {
	if id, ok := ctx.Value(contextKey{}).(string); ok {
		return id
	}
	return ""
}
