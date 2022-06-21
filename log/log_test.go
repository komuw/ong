package log

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/akshayjshah/attest"
)

func TestLogger(t *testing.T) {
	t.Parallel()

	t.Run("info level does not do anything", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3
		l := New(context.Background(), w, maxMsgs, true)
		l.Info(F{"one": "one"})

		attest.Zero(t, w.String())
	})

	t.Run("error logs immediately", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3
		l := New(context.Background(), w, maxMsgs, true)
		msg := "oops, Houston we got 99 problems."
		l.Error(F{"err": msg})

		attest.True(t, strings.Contains(w.String(), msg))
	})

	t.Run("info logs are flushed on error", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3
		l := New(context.Background(), w, maxMsgs, true)

		infoMsg := "hello world"
		l.Info(F{"what": infoMsg})
		errMsg := "oops, Houston we got 99 problems."
		l.Error(F{"err": errMsg})

		attest.True(t, strings.Contains(w.String(), infoMsg))
		attest.True(t, strings.Contains(w.String(), errMsg))
	})

	t.Run("logId added", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3
		l := New(context.Background(), w, maxMsgs, true)

		infoMsg := "hello world"
		l.Info(F{"what": infoMsg})
		errMsg := "oops, Houston we got 99 problems."
		l.Error(F{"err": errMsg})

		id := getLogId(l.ctx)
		attest.True(t, strings.Contains(w.String(), id))
	})

	t.Run("logs are rotated", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3
		l := New(context.Background(), w, maxMsgs, true)

		for i := 0; i <= (maxMsgs + 4); i++ {
			infoMsg := "hello world" + " : " + fmt.Sprint(i)
			l.Info(F{"what": infoMsg})
		}
		errMsg := "oops, Houston we got 99 problems."
		l.Error(F{"err": errMsg})

		attest.False(t, strings.Contains(w.String(), "hello world : 1"))
		attest.False(t, strings.Contains(w.String(), "hello world : 2"))
		attest.False(t, strings.Contains(w.String(), "hello world : 5"))
		attest.True(t, strings.Contains(w.String(), "hello world : 6"))
		attest.True(t, strings.Contains(w.String(), "hello world : 7"))
		attest.True(t, strings.Contains(w.String(), errMsg))
	})
}
