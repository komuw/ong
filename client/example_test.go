package client_test

import (
	"os"

	"github.com/komuw/ong/client"
	"github.com/komuw/ong/log"
)

func ExampleSafe() {
	l := log.New(os.Stdout, 7)

	cli := client.Safe(l)
	// This is the AWS metadata url.
	url := "http://169.254.169.254/latest/meta-data"
	_, _ = cli.Get(url)

	// This will log something like:
	// {"durationMS":0,"err":"dial tcp 169.254.169.254:80: ong/client: address 169.254.169.254 IsLinkLocalUnicast","level":"error","logID":"yHSDRXAJP7QKx3AJQKNt7w","method":"GET","msg":"http_client","pid":325431,"timestamp":"2022-12-08T08:43:42.151691969Z","url":"http://169.254.169.254/latest/meta-data"}
}
