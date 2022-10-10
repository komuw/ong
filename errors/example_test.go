package errors_test

import (
	"fmt"

	"github.com/komuw/ong/errors"
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
