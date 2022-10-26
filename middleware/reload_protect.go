package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/komuw/ong/cookie"
	"golang.org/x/exp/slices"
)

// TODO: docs.
// ReloadProtect blah against Form blah
func ReloadProtect(wrappedHandler http.HandlerFunc, domain string) http.HandlerFunc {
	safeMethods := []string{
		// safe methods under rfc7231: https://datatracker.ietf.org/doc/html/rfc7231#section-4.2.1
		http.MethodGet,
		http.MethodHead,
		http.MethodOptions,
		http.MethodTrace,
	}
	return func(w http.ResponseWriter, r *http.Request) {
		theCookie := fmt.Sprintf(
			"ong_form_reload_protect-%s",
			strings.ReplaceAll(r.URL.EscapedPath(), "/", ""),
		)

		if !slices.Contains(safeMethods, r.Method) {
			// This could be a http POST/DELETE/etc
			defer func() {
				cookie.Set(
					w,
					theCookie,
					"YES",
					domain,
					3*time.Hour,
					false,
				)
			}()

			gotCookie, _ := r.Cookie(theCookie)
			if gotCookie != nil {
				// It means that the form had been submitted before.

				cookie.Delete(
					w,
					theCookie,
					domain,
				)
				http.Redirect(
					w,
					r,
					r.URL.String(),
					// http 303(StatusSeeOther) is guaranteed by the spec to always use http GET.
					// https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/303
					http.StatusSeeOther,
				)
				return
			}
		}

		// // TODO: check if request method is safe

		// ct, _, err := mime.ParseMediaType(r.Header.Get(ctHeader))
		// if err == nil && (ct == formUrlEncoded || ct == multiformData) {
		// 	// For POST requests that;
		// 	// - are not form data.
		// 	// - have no cookies.
		// 	// - are not using http authentication.
		// 	// then it is okay to not validate csrf for them.
		// 	// This is especially useful for REST API endpoints.
		// 	// see: https://github.com/komuw/ong/issues/76
		// 	break
		// }

		wrappedHandler(w, r)
	}
}
