package log

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/akshayjshah/attest"
	"github.com/komuw/goweb/errors"
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
		l.Error(errors.New(msg))

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
		l.Error(errors.New(errMsg))

		attest.True(t, strings.Contains(w.String(), infoMsg))
		attest.True(t, strings.Contains(w.String(), errMsg))
	})

	t.Run("neccesary fields added", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3
		l := New(context.Background(), w, maxMsgs, true)

		{
			infoMsg := "hello world"
			l.Info(F{"what": infoMsg})
			l.Error(errors.New("bad"))

			id := getLogId(l.ctx)
			attest.True(t, strings.Contains(w.String(), id))
			attest.True(t, strings.Contains(w.String(), "level"))
			attest.True(t, strings.Contains(w.String(), "stack"))
			attest.True(t, strings.Contains(w.String(), "err"))
			attest.False(t, strings.Contains(w.String(), "line")) // line not added
		}

		{
			l = l.WithCaller()
			l.Info(F{"name": "john"})
			errMsg := "kimeumana"
			l.Error(errors.New(errMsg))

			id := getLogId(l.ctx)
			attest.True(t, strings.Contains(w.String(), id))
			attest.True(t, strings.Contains(w.String(), "level"))
			attest.True(t, strings.Contains(w.String(), "stack"))
			attest.True(t, strings.Contains(w.String(), "err"))
			attest.True(t, strings.Contains(w.String(), "line")) // line added
		}
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
		l.Error(errors.New(errMsg))

		attest.False(t, strings.Contains(w.String(), "hello world : 1"))
		attest.False(t, strings.Contains(w.String(), "hello world : 2"))
		attest.False(t, strings.Contains(w.String(), "hello world : 5"))
		attest.True(t, strings.Contains(w.String(), "hello world : 6"))
		attest.True(t, strings.Contains(w.String(), "hello world : 7"))
		attest.True(t, strings.Contains(w.String(), errMsg))
	})

	t.Run("WithCtx invalidates buffer", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3
		l := New(context.Background(), w, maxMsgs, true)
		{
			for i := 0; i <= (maxMsgs); i++ {
				infoMsg := "hello world" + " : " + fmt.Sprint(i)
				l.Info(F{"what": infoMsg})
			}
			attest.False(t, strings.Contains(w.String(), "hello world : 0"))
			attest.False(t, strings.Contains(w.String(), "hello world : 1"))
			attest.False(t, strings.Contains(w.String(), "hello world : 2"))
			attest.False(t, strings.Contains(w.String(), "hello world : 3"))
			attest.False(t, strings.Contains(w.String(), "hello world : 4"))
		}

		{
			l = l.WithCtx(context.Background())
			errMsg := "oops, Houston we got 99 problems."
			l.Error(errors.New(errMsg))

			attest.False(t, strings.Contains(w.String(), "hello world : 0"))
			attest.False(t, strings.Contains(w.String(), "hello world : 1"))
			attest.False(t, strings.Contains(w.String(), "hello world : 2"))
			attest.False(t, strings.Contains(w.String(), "hello world : 3"))
			attest.False(t, strings.Contains(w.String(), "hello world : 4"))
			attest.True(t, strings.Contains(w.String(), errMsg))
		}
	})

	t.Run("WithCaller does not invalidate buffer", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3
		l := New(context.Background(), w, maxMsgs, true)
		{
			for i := 0; i <= (maxMsgs); i++ {
				infoMsg := "hello world" + " : " + fmt.Sprint(i)
				l.Info(F{"what": infoMsg})
			}
			attest.False(t, strings.Contains(w.String(), "hello world : 0"))
			attest.False(t, strings.Contains(w.String(), "hello world : 1"))
			attest.False(t, strings.Contains(w.String(), "hello world : 2"))
			attest.False(t, strings.Contains(w.String(), "hello world : 3"))
		}

		{
			l = l.WithCaller()
			errMsg := "oops, Houston we got 99 problems."
			l.Error(errors.New(errMsg))

			attest.False(t, strings.Contains(w.String(), "hello world : 0"))
			attest.False(t, strings.Contains(w.String(), "hello world : 1"))
			attest.False(t, strings.Contains(w.String(), "hello world : 2"))
			attest.True(t, strings.Contains(w.String(), "hello world : 3"))
			attest.True(t, strings.Contains(w.String(), errMsg))
		}
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		w := &bytes.Buffer{}
		maxMsgs := 3
		l := New(context.Background(), w, maxMsgs, true)

		tokens := []string{
			"a", "aa", "aaa", "aaron", "ab", "abandoned", "abc", "aberdeen", "abilities", "ability", "able", "aboriginal", "abortion",
			"about", "above", "abraham", "abroad", "abs", "absence", "absent", "absolute", "absolutely", "absorption", "abstract",
			"abstracts", "abu", "abuse", "ac", "academic", "academics", "academy", "acc", "accent", "accept", "acceptable", "acceptance",
			"accepted", "accepting", "accepts", "access", "accessed", "accessibility", "accessible", "accessing", "accessories",
			"accessory", "accident", "accidents", "accommodate", "accommodation", "accommodations", "accompanied", "accompanying",
			"accomplish", "accomplished", "accordance", "according", "accordingly", "account", "accountability", "accounting", "accounts",
			"accreditation", "accredited", "accuracy", "accurate", "accurately", "accused", "acdbentity", "ace",
		}

		for _, tok := range tokens {
			go func(t string) {
				l.Info(F{"one": "one" + t})
			}(tok)
		}

		for _, tok := range tokens {
			go func(t string) {
				l.Error(errors.New("bad" + t))
			}(tok)
		}

		for _, tok := range tokens {
			go func(t string) {
				l.Error(errors.New("bad-two" + t))
			}(tok)
		}

		wg := &sync.WaitGroup{}
		for _, tok := range tokens {
			wg.Add(1)
			go func(t string) {
				l.Info(F{"four": "four" + t})
				wg.Done()
			}(tok)
		}
		wg.Wait()
	})
}
