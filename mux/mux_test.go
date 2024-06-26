package mux

import (
	"bytes"
	"context"
	"net/http"
	"testing"

	"github.com/komuw/ong/config"
	"github.com/komuw/ong/log"
	"go.akshayshah.org/attest"
)

func TestNew(t *testing.T) {
	l := log.New(context.Background(), &bytes.Buffer{}, 500)

	routes := func() []Route {
		return []Route{
			NewRoute("/home", MethodGet, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})),
			NewRoute("/home/", MethodAll, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})),
		}
	}

	// There are other tests in internal/mx
	t.Run("conflict detected", func(t *testing.T) {
		rtz := []Route{}
		rtz = append(rtz, tarpitRoutes()...)
		rtz = append(rtz, routes()...)

		attest.Panics(t, func() {
			_ = New(config.DevOpts(l, "secretKey12@34String"), nil, rtz...)
		})
	})

	t.Run("okay", func(t *testing.T) {
		rtz := []Route{
			NewRoute("/home", MethodGet, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})),
			NewRoute("/health/", MethodAll, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})),
		}
		// does not panic.
		_ = New(config.DevOpts(l, "secretKey12@34String"), nil, rtz...)
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
