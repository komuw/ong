package cookie_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/komuw/ong/cookie"
)

type shoppingCart struct {
	ItemName string
	Price    uint8
}

func shoppingCartHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookieName := "cart"
		key := "superSecret"
		item := shoppingCart{ItemName: "shoe", Price: 89}

		b, err := json.Marshal(item)
		if err != nil {
			panic(err)
		}

		cookie.SetEncrypted(
			r,
			w,
			cookieName,
			string(b),
			"example.com",
			2*time.Hour,
			key,
		)

		fmt.Fprint(w, "thanks for shopping!")
	}
}

func ExampleSetEncrypted() {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/shop", nil)
	shoppingCartHandler().ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		panic("unexcpected")
	}

	fmt.Println(res.Cookies()[0].Name)

	// Output:
	// cart
}
