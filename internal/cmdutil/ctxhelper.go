package cmdutil

import "context"

func withValue(parent context.Context, key, val any) context.Context {
	return context.WithValue(parent, key, val)
}
