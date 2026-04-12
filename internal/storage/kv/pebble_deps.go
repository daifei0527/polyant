// Package kv provides key-value storage implementations
package kv

import (
	// Pebble is a high-performance key-value store from CockroachDB
	// This import ensures the dependency is tracked for the upcoming PebbleStore implementation
	_ "github.com/cockroachdb/pebble"
)
