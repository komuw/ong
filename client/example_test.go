package client_test

import (
	"context"
	"os"

	"github.com/komuw/ong/client"
	"github.com/komuw/ong/log"
)

func ExampleClient_Get() {
	ctx := context.Background()
	l := log.New(ctx, os.Stdout, 7)

	cli := client.SafeClient(l)
	_, _ = cli.Get(ctx, "https://ajmsmsYnns-bad-domain.com")

	// This will log:
	// {"level":"info","logID":"D2MH3e3BqZmRgm3WK8yK7Q","method":"GET","msg":"http_client","pid":2102616,"process":"request","timestamp":"2022-09-16T09:39:55.423743309Z","url":"https://ajmsmsYnns-bad-domain.com"}
	// {"durationMS":339,"err":"Get \"https://ajmsmsYnns-bad-domain.com\": dial tcp: lookup ajmsmsYnns-bad-domain.com on 127.0.0.53:53: no such host","level":"error","logID":"D2MH3e3BqZmRgm3WK8yK7Q","method":"GET","msg":"http_client","pid":2102616,"process":"response","timestamp":"2022-09-16T09:39:55.762989726Z","url":"https://ajmsmsYnns-bad-domain.com"}
}
