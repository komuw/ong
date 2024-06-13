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
	tarpitHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	routes := []Route{}

	for _, uri := range []string{
		// CMS

		"/images/",
		"/joomla/",
		"/libraries/joomla/",
		"/administrator/",
		"/components/",
		"/templates/",
		"/includes/",
		"/modules/",
		"/plugins/",
		"/drupal/",
		"/Drupal.php",

		// OTHERS
		"/echo.php",
		"/composer.php",
		"/uploader.php",
		"/shell.php",
		"/freenode-proxy-checker.txt",
		"/w00tw00t.at.blackhats.romanian.anti-sec:)",
	} {
		uri := uri
		routes = append(
			routes,
			NewRoute(
				uri,
				MethodAll,
				tarpitHandler,
			),
		)

	}

	return routes
}
