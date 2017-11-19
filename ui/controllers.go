package ui

import (
	"fmt"
	"net/http"
)

// RootHandler renders the template for the root page and returns.
func RootHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, err := RenderRoot()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprintf(w, "error: %s", err.Error())
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "%s", string(p))
		return
	})
}
