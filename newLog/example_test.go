package log_test

import (
	"context"
	"errors"
	"os"

	log "github.com/komuw/ong/newLog"
)

func ExampleNew() {
	l := log.New(os.Stdout, 1000)

	hey := func(ctx context.Context) {
		logger := l(ctx)

		logger.Info("sending email", "email", "jane@example.com")
		logger.Error("fail", errors.New("sending email failed."), "email", "jane@example.com")
	}

	hey(context.Background())

	// example output:
	//   {"email":"jane@example.com","level":"info","logID":"r73RdRZEExH7cnax2faY7A","msg":"sending email","timestamp":"2022-09-16T12:56:05.471496845Z"}
	//   {"email":"jane@example.com","err":"sending email failed.","level":"error","logID":"r73RdRZEExH7cnax2faY7A","timestamp":"2022-09-16T12:56:05.471500752Z"}
}
