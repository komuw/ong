package client

import (
	"fmt"
	"testing"
)

func TestClient(t *testing.T) {
	t.Parallel()

	t.Run("ssrf safe", func(t *testing.T) {
		t.Parallel()

		// http://[fd00:ec2::254]/latest/meta-data/
		url := "http://[fd00:ec2::254]/latest/meta-data/" //"http://169.254.169.254/latest/meta-data/" //"https://www.google.com"

		ssrfSafe := true
		cli := getClient(ssrfSafe)

		res, err := cli.Get(url)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println("\n\t res, ssrfSafe: ", res.StatusCode, ssrfSafe)
	})

	t.Run("ssrf unsafe", func(t *testing.T) {
		t.Parallel()

		// http://[fd00:ec2::254]/latest/meta-data/
		url := "http://[fd00:ec2::254]/latest/meta-data/" //"http://169.254.169.254/latest/meta-data/" //"https://www.google.com"

		ssrfSafe := false
		cli := getClient(ssrfSafe)

		res, err := cli.Get(url)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println("\n\t res, ssrfSafe: ", res.StatusCode, ssrfSafe)
	})
}
