package middleware

import "net/http"

// Chain applies mws to h in order, so Chain(h, A, B) behaves as A(B(h)) —
// i.e. A runs first when a request comes in.
func Chain(h http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	
	return h
}
