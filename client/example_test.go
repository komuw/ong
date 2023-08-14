package client_test

import (
	"context"
	"os"

	"github.com/komuw/ong/client"
	"github.com/komuw/ong/log"
)

func ExampleSafe() {
	l := log.New(context.Background(), os.Stdout, 7)

	cli := client.Safe(l)
	// This is the AWS metadata url.
	url := "http://169.254.169.254/latest/meta-data"
	_, _ = cli.Get(url)

	// This will log something like:
	// {"time":"2023-02-03T18:54:13.300137216Z","level":"ERROR","source":"client.go:108","msg":"http_client","err":"dial tcp 169.254.169.254:80: ong/client: address 169.254.169.254 IsLinkLocalUnicast","method":"GET","url":"http://169.254.169.254/latest/meta-data","durationMS":0,"logID":"DYGY3eedgARMAyXZ"}
}
