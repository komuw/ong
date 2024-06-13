package mux

import (
	"net/http"
	"testing"

	"github.com/komuw/ong/config"
	"go.akshayshah.org/attest"
)

func TestNew(t *testing.T) {
	routes := func() []Route {
		r1 := NewRoute("/home", MethodGet, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		r2 := NewRoute("/home/", MethodAll, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		return []Route{r1, r2}
	}

	t.Run("conflict detected", func(t *testing.T) {
		attest.Panics(t, func() {
			_ = New(config.DevOpts(nil, "secretKey12@34String"), nil, routes()...)
		})
	})
}
