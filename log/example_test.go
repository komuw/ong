package log_test

import (
	"errors"
	"os"

	"github.com/komuw/ong/log"
)

func ExampleLogger_Error() {
	l := log.New(os.Stdout, 1000)

	l.Info(log.F{"msg": "sending email", "email": "jane@example.com"})
	l.Error(errors.New("sending email failed."), log.F{"email": "jane@example.com"})

	// example output:
	//   {"email":"jane@example.com","level":"info","logID":"r73RdRZEExH7cnax2faY7A","msg":"sending email","timestamp":"2022-09-16T12:56:05.471496845Z"}
	//   {"email":"jane@example.com","err":"sending email failed.","level":"error","logID":"r73RdRZEExH7cnax2faY7A","timestamp":"2022-09-16T12:56:05.471500752Z"}
}
