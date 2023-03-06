package errors_test

import (
	"fmt"
	"os"

	"github.com/komuw/ong/errors"

	"golang.org/x/exp/slices"
)

const expectedUser = "admin"

func login(user string) error {
	if user == expectedUser {
		return nil
	}

	return errors.New("invalid user")
}

func Example_stackTraceFormatting() {
	err := login("badGuy")
	fmt.Printf("%+v", err)
}

func ExampleWrap() {
	opener := func(p string) error {
		_, err := os.Open(p)
		if err != nil {
			return errors.Wrap(err)
		}
		return nil
	}

	fmt.Printf("%+v", opener("/this/file/does/not/exist.txt"))
}

func ExampleDwrap() {
	fetchUser := func(u string) (errp error) {
		defer errors.Dwrap(&errp)

		users := []string{"John", "Alice", "Kamau"}
		if !slices.Contains(users, u) {
			return fmt.Errorf("user %s not found", u)
		}

		return nil
	}

	e := fetchUser("Emmy")
	fmt.Printf("%+#v", e)
}
