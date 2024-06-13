package mux

import (
	"net/http"
	"testing"

	"github.com/komuw/ong/config"
)

func TestNew(t *testing.T) {
	routes := func() []Route {
		return []Route{
			NewRoute("/home", MethodGet, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})),
			NewRoute("/home/", MethodAll, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})),
		}
	}

	t.Run("conflict detected", func(t *testing.T) {
		rtz := []Route{}
		rtz = append(rtz, tarpitRoutes()...)
		rtz = append(rtz, routes()...)

		// attest.Panics(t, func() {
		_ = New(config.DevOpts(nil, "secretKey12@34String"), nil, rtz...)
		// })
	})
}

func tarpitRoutes() []Route {
	return []Route{
		NewRoute(
			"/libraries/joomla/",
			MethodAll,
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
		),
	}
}
