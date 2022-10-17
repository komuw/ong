package sess_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/komuw/ong/middleware"
	"github.com/komuw/ong/sess"
)

const (
	secretKey = "some-secretKey"
	domain    = "example.com"
)

func loginHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mySession := map[string]string{
			"name":           "John Doe",
			"favorite_color": "red",
			"height":         "5 feet 6 inches",
		}
		sess.SetM(r, mySession)

		fmt.Fprint(w, "welcome again.")
	}
}

func ExampleSetM() {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	handler := middleware.Session(loginHandler(), secretKey, domain)
	handler.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		panic("unexcpected")
	}

	fmt.Println(res.Cookies()[0])
}
