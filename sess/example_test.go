package sess_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/komuw/ong/log"
	"github.com/komuw/ong/middleware"
	"github.com/komuw/ong/sess"
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
	l := log.New(os.Stdout, 100)(context.Background())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	handler := middleware.Get(
		loginHandler(),
		middleware.WithOpts("example.com", 443, "super-h@rd-Pa$1word", middleware.DirectIpStrategy, l),
	)
	handler.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		panic("unexcpected")
	}

	fmt.Println(res.Cookies()[0])
}
