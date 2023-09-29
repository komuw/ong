package sync_test

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/komuw/ong/sync"
)

// JustErrors illustrates the use of a Group in place of a sync.WaitGroup to
// simplify goroutine counting and error handling.
// This example is derived from the sync.WaitGroup example at https://golang.org/pkg/sync/#example_WaitGroup.
func ExampleWaitGroup_justErrors() {
	wg, ctx := sync.New(context.Background(), 2)

	urls := []string{
		"http://www.example.org/",
		"http://www.example.com/",
		"http://www.nonExistentDomainName.com/",
	}

	funcs := []func() error{}
	for _, url := range urls {
		url := url // https://golang.org/doc/faq#closures_and_goroutines
		funcs = append(
			funcs,
			func() error {
				// Fetch the URL.
				ct, cancel := context.WithTimeout(ctx, 4*time.Second)
				defer cancel()

				req, err := http.NewRequestWithContext(ct, http.MethodGet, url, nil)
				if err != nil {
					return err
				}
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					return err
				}
				defer resp.Body.Close()
				return err
			},
		)
	}

	funcs = append(
		funcs,
		func() error {
			return nil
		},
	)

	err := wg.Go(funcs...)
	fmt.Printf("\n\t err: %v. cause: %v\n\n", err, context.Cause(ctx))
}
