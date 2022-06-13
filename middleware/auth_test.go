package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/akshayjshah/attest"
)

func protectedHandler(msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, msg)
	}
}

func TestBasicAuth(t *testing.T) {
	t.Parallel()

	msg := "hello"
	user := "some-user"
	passwd := "some-passwd"
	wrappedHandler := BasicAuth(protectedHandler(msg), user, passwd)

	tests := []struct {
		name     string
		user     string
		passwd   string
		wantCode int
	}{
		{
			name:     "no credentials",
			user:     "",
			passwd:   "",
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "user not match",
			user:     "fakeUser",
			passwd:   passwd,
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "passwd not match",
			user:     user,
			passwd:   "fakePasswd",
			wantCode: http.StatusUnauthorized,
		},
		{
			name:     "success",
			user:     user,
			passwd:   passwd,
			wantCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/someUri", nil)
			if tt.user != "" || tt.passwd != "" {
				r.SetBasicAuth(tt.user, tt.passwd)
			}
			wrappedHandler.ServeHTTP(rec, r)

			res := rec.Result()
			defer res.Body.Close()

			attest.Equal(t, res.StatusCode, tt.wantCode)
		})
	}
}
