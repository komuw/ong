package log_test

import (
	"context"
	"errors"
	"os"

	"github.com/komuw/ong/log"
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
	//   {"time":"2023-02-03T11:26:47.460792396Z","level":"INFO","source":"main.go:17","msg":"sending email","email":"jane@example.com","logID":"DQTXGs3HM8Xgx3yt"}
	//   {"time":"2023-02-03T11:26:47.46080217Z","level":"ERROR","source":"main.go:18","msg":"fail","err":"sending email failed.","email":"jane@example.com","logID":"DQTXGs3HM8Xgx3yt"}
}
