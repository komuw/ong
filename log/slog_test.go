package log

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"testing/slogtest"

	"go.akshayshah.org/attest"
)

// Check that our handler is conformant with log/slog expectations.
// Taken from https://github.com/golang/go/blob/go1.22.0/src/log/slog/slogtest_test.go#L18-L26
//
// We test this handler even though acording to Jonathan Amsterdam(jba):
// `I think wrapping handlers like those that don't actually affect the format don't need testing/slogtest`
// https://github.com/golang/go/issues/61706#issuecomment-1674498394
func TestSlogtest(t *testing.T) {
	t.Parallel()

	{ // sanity check that logger works.
		var buf bytes.Buffer
		l := New(context.Background(), &buf, 300)

		l.Error("hello world", "err", "someBadError")
		if !strings.Contains(buf.String(), "hello world") {
			t.Fatal("expected it to log")
		}
		t.Log("hey::: ", buf.String())
	}

	parseJSON := func(bs []byte) (map[string]any, error) {
		t.Helper()

		var m map[string]any
		if err := json.Unmarshal(bs, &m); err != nil {
			return nil, err
		}
		return m, nil
	}

	tests := []struct {
		name      string
		parseFunc func([]byte) (map[string]any, error)
		maxSize   int
	}{
		{
			name:      "JSON",
			parseFunc: parseJSON,
			maxSize:   3,
		},
		{
			name:      "JSON",
			parseFunc: parseJSON,
			maxSize:   4_034,
		},
		{
			name:      "JSON",
			parseFunc: parseJSON,
			maxSize:   100_295,
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(fmt.Sprintf("%s-%d", tt.name, tt.maxSize), func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer

			results := func(t *testing.T) map[string]any {
				m := map[string]any{}
				if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
					t.Fatal(err)
				}
				return m
			}

			slogtest.Run(
				t,
				func(*testing.T) slog.Handler {
					buf.Reset()

					l := New(context.Background(), &buf, tt.maxSize)
					h := l.Handler()
					{
						underlyingHandler, ok := h.(*handler)
						attest.Equal(t, ok, true)
						underlyingHandler.forceFlush = struct{}{}
					}

					return h
				},
				results,
			)
		})
	}
}
