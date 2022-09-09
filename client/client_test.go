package client

import (
	"net/http"
	"testing"

	"github.com/akshayjshah/attest"
)

func TestClient(t *testing.T) {
	t.Parallel()

	// aws metadata api.
	urlsInPrivate := []string{
		"http://[fd00:ec2::254]/latest/meta-data/",
		"http://169.254.169.254/latest/meta-data/",
	}

	urlsInPublic := []string{
		"http://www.example.com",
		"https://www.example.com",
	}

	t.Run("ssrf safe", func(t *testing.T) {
		t.Parallel()

		ssrfSafe := true

		for _, url := range urlsInPrivate {
			cli := getClient(ssrfSafe)
			_, err := cli.Get(url)
			attest.Error(t, err)
			attest.Subsequence(t, err.Error(), "is not a public IP address")
		}

		for _, url := range urlsInPublic {
			cli := getClient(ssrfSafe)
			res, err := cli.Get(url)
			attest.Ok(t, err)
			attest.Equal(t, res.StatusCode, http.StatusOK)
		}
	})

	// t.Run("ssrf unsafe", func(t *testing.T) {
	// 	t.Parallel()

	// 	// http://[fd00:ec2::254]/latest/meta-data/
	// 	url := "http://[fd00:ec2::254]/latest/meta-data/" //"http://169.254.169.254/latest/meta-data/" //"https://www.google.com"

	// 	ssrfSafe := false
	// 	cli := getClient(ssrfSafe)

	// 	res, err := cli.Get(url)
	// 	if err != nil {
	// 		t.Fatal(err)
	// 	}
	// 	fmt.Println("\n\t res, ssrfSafe: ", res.StatusCode, ssrfSafe)
	// })
}
