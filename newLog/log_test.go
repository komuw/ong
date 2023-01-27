package log

import (
	"fmt"
	"testing"

	"github.com/akshayjshah/attest"
	"golang.org/x/exp/slog"
)

func TestCircleBuf(t *testing.T) {
	t.Parallel()

	t.Run("it stores", func(t *testing.T) {
		t.Parallel()

		maxSize := 4
		c := newCirleBuf(maxSize)

		c.store(slog.Attr{Key: "msg", Value: slog.StringValue("one")})
		c.store(slog.Attr{Key: "msg", Value: slog.StringValue("two")})

		attest.Equal(t, c.buf[0].Key, "msg")
		attest.Equal(t, c.buf[0].Value.String(), "one")

		attest.Equal(t, c.buf[1].Key, "msg")
		attest.Equal(t, c.buf[1].Value.String(), "two")

		attest.Equal(t, len(c.buf), 2)
		attest.Equal(t, cap(c.buf), 4)
	})

	t.Run("does not exceed maxsize", func(t *testing.T) {
		t.Parallel()

		maxSize := 8
		c := newCirleBuf(maxSize)
		for i := 0; i <= (13 * maxSize); i++ {
			x := fmt.Sprint(i)
			c.store(slog.Attr{Key: x, Value: slog.StringValue(x)})

			attest.True(t, len(c.buf) <= maxSize)
			attest.True(t, cap(c.buf) <= maxSize)
		}
		attest.True(t, len(c.buf) <= maxSize)
		attest.True(t, cap(c.buf) <= maxSize)
	})

	// t.Run("clears oldest first", func(t *testing.T) {
	// 	t.Parallel()

	// 	maxSize := 5
	// 	c := newCirleBuf(maxSize)
	// 	for i := 0; i <= (6 * maxSize); i++ {
	// 		x := fmt.Sprint(i)
	// 		c.store(F{"msg": x})
	// 		attest.True(t, len(c.buf) <= maxSize)
	// 		attest.True(t, cap(c.buf) <= maxSize)
	// 	}
	// 	attest.True(t, len(c.buf) <= maxSize)
	// 	attest.True(t, cap(c.buf) <= maxSize)

	// 	val1, ok := c.buf[1]["msg"].(string)
	// 	attest.True(t, ok)
	// 	attest.Equal(t, val1, "29")
	// 	val2, ok := c.buf[2]["msg"].(string)
	// 	attest.True(t, ok)
	// 	attest.Equal(t, val2, "30")
	// })

	// t.Run("reset", func(t *testing.T) {
	// 	t.Parallel()

	// 	maxSize := 80
	// 	c := newCirleBuf(maxSize)
	// 	for i := 0; i <= (13 * maxSize); i++ {
	// 		x := fmt.Sprint(i)
	// 		c.store(F{x: x})
	// 		attest.True(t, len(c.buf) <= maxSize)
	// 		attest.True(t, cap(c.buf) <= maxSize)
	// 	}
	// 	attest.True(t, len(c.buf) <= maxSize)
	// 	attest.True(t, cap(c.buf) <= maxSize)

	// 	c.reset()

	// 	attest.Equal(t, len(c.buf), 0)
	// 	attest.Equal(t, cap(c.buf), maxSize)
	// })
}
