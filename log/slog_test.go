package log

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// Check that our handler is conformant with log/slog expectations.
// Taken from https://github.com/golang/go/blob/go1.21.0/src/log/slog/slogtest_test.go#L18-L26
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

	parseLines := func(src []byte, parse func([]byte) (map[string]any, error)) ([]map[string]any, error) {
		t.Helper()

		var records []map[string]any
		for _, line := range bytes.Split(src, []byte{'\n'}) {
			if len(line) == 0 {
				continue
			}
			m, err := parse(line)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", string(line), err)
			}
			records = append(records, m)
		}
		return records, nil
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
			l := New(context.Background(), &buf, tt.maxSize)
			handler := l.Handler()

			results := func() []map[string]any {
				ms, err := parseLines(buf.Bytes(), tt.parseFunc)
				if err != nil {
					t.Fatal(err)
				}
				return ms
			}

			err := testHandler(handler, results)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
