package client_test

import (
	"context"
	"os"

	"github.com/komuw/ong/client"
	"github.com/komuw/ong/log"
)

func ExampleClient_Get() {
	l := log.New(os.Stdout, 7)

	cli := client.Safe(l)
	// This is the AWS metadata url.
	url := "http://169.254.169.254/latest/meta-data"
	_, _ = cli.Get(context.Background(), url)

	// This will log something like:
	// {"level":"info","logID":"Z5X7qXm8HkT8kZ83xQyrrQ","method":"GET","msg":"http_client","pid":11776,"process":"request","timestamp":"2022-10-09T12:03:33.851543383Z","url":"http://169.254.169.254/latest/meta-data"}
	// {"durationMS":0,"err":"Get \"http://169.254.169.254/latest/meta-data\": dial tcp 169.254.169.254:80: ong/client: address 169.254.169.254 IsLinkLocalUnicast","level":"error","logID":"Z5X7qXm8HkT8kZ83xQyrrQ","method":"GET","msg":"http_client","pid":11776,"process":"response","timestamp":"2022-10-09T12:03:33.851889217Z","url":"http://169.254.169.254/latest/meta-data"}
}
