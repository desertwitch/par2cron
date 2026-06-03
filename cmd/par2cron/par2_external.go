//go:build !embed_par2

package main

// setupPar2 and the returned cleanup function are no-ops for default builds.
func setupPar2() (func(), error) {
	return func() {}, nil
}
