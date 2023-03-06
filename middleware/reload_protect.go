package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/komuw/ong/cookie"

	"golang.org/x/exp/slices"
)

const reloadProtectCookiePrefix = "ong_form_reload_protect"

// reloadProtector is a middleware that attempts to provides protection against a form re-submission when a user reloads/refreshes an already submitted web page/form.
//
// If such a situation is detected; this middleware will issue a http GET redirect to the same url.
func reloadProtector(wrappedHandler http.HandlerFunc, domain string) http.HandlerFunc {
	safeMethods := []string{
		// safe methods under rfc7231: https://datatracker.ietf.org/doc/html/rfc7231#section-4.2.1
		http.MethodGet,
		http.MethodHead,
		http.MethodOptions,
		http.MethodTrace,
	}
	return func(w http.ResponseWriter, r *http.Request) {
		// It is possible for one to send a form without having added the requiste form http header.
		if !slices.Contains(safeMethods, r.Method) {
			// This could be a http POST/DELETE/etc

			theCookie := fmt.Sprintf("%s-%s",
				reloadProtectCookiePrefix,
				strings.ReplaceAll(r.URL.EscapedPath(), "/", ""),
			)

			// todo: should we check if gotCookie.MaxAge > 0
			gotCookie, err := r.Cookie(theCookie)
			if err == nil && gotCookie != nil {
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
			} else {
				cookie.Set(
					w,
					theCookie,
					"YES",
					domain,
					1*time.Hour,
					false,
				)
			}
		}

		wrappedHandler(w, r)
	}
}
