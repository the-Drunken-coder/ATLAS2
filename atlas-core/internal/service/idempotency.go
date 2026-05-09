package service

// IdempotencyOption attaches an idempotency key to a mutating function call.
// When provided, the function tries to claim the key before performing the
// operation. A repeated call with the same key against the same resource
// returns nil (the original effect). A call with the same key against a
// different resource returns model.ErrConflict.
type IdempotencyOption func(*idempotencyOptions)

type idempotencyOptions struct {
	key string
}

// WithIdempotencyKey returns an IdempotencyOption that scopes a mutation to
// the given client-supplied key. Empty keys disable the check.
func WithIdempotencyKey(key string) IdempotencyOption {
	return func(o *idempotencyOptions) { o.key = key }
}

func resolveIdempotency(opts []IdempotencyOption) idempotencyOptions {
	var o idempotencyOptions
	for _, fn := range opts {
		if fn == nil {
			continue
		}
		fn(&o)
	}
	return o
}
