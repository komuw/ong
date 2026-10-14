package cry

import (
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/komuw/ong/internal/tst"
	"go.akshayshah.org/attest"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	// call flag.Parse() here if TestMain uses flags
	goleak.VerifyTestMain(m)
}

func TestEnc(t *testing.T) {
	t.Parallel()

	t.Run("new", func(t *testing.T) {
		t.Parallel()

		// okay key
		key := tst.SecretKey()
		_ = New(key)

		// short key
		attest.Panics(t, func() {
			_ = New("hi")
		})
	})

	t.Run("encrypt/decrypt", func(t *testing.T) {
		t.Parallel()

		msgToEncrypt := "hello world!"
		key := tst.SecretKey()
		enc := New(key)

		encryptedMsg := enc.Encrypt(msgToEncrypt)

		decryptedMsg, err := enc.Decrypt(encryptedMsg)
		attest.Ok(t, err)

		attest.Equal(t, string(decryptedMsg), msgToEncrypt)
	})

	t.Run("encrypt/decrypt base64", func(t *testing.T) {
		t.Parallel()

		msgToEncrypt := "hello world!"
		key := tst.SecretKey()
		enc := New(key)

		token := enc.EncryptEncode(msgToEncrypt)

		decryptedMsg, err := enc.DecryptDecode(token)
		attest.Ok(t, err)
		attest.Equal(t, decryptedMsg, msgToEncrypt)
	})

	t.Run("short input does not panic", func(t *testing.T) {
		t.Parallel()

		enc := New(tst.SecretKey())

		for n := 0; n <= 40; n++ {
			msg := []byte(strings.Repeat("A", n))

			_, err := enc.Decrypt(msg)
			attest.Error(t, err)

			_, err = enc.DecryptDecode(string(msg))
			attest.Error(t, err)
		}
	})

	t.Run("encrypt same msg is unique", func(t *testing.T) {
		t.Parallel()

		// This is a useful property especially in how we use it in csrf protection
		// against breachattack.

		msgToEncrypt := "hello world!"
		key := tst.SecretKey()
		enc := New(key)

		encryptedMsg := enc.Encrypt(msgToEncrypt)

		var em []byte
		for range 4 {
			em = enc.Encrypt(msgToEncrypt)
			if slices.Equal(encryptedMsg, em) {
				t.Fatal("slices should not be equal")
			}
		}

		decryptedMsg, err := enc.Decrypt(encryptedMsg)
		attest.Ok(t, err)
		attest.Equal(t, string(decryptedMsg), msgToEncrypt)

		decryptedMsgForEm, err := enc.Decrypt(em)
		attest.Ok(t, err)
		attest.Equal(t, string(decryptedMsgForEm), msgToEncrypt)
	})

	t.Run("same input key will always be able to encrypt and decrypt", func(t *testing.T) {
		t.Parallel()

		// This is a useful property especially in the csrf implementation.
		// A csrf token that was encrypted today, should be able to be decrypted tomorrow
		// even if the server was restarted; so long as the same key is re-used.

		msgToEncrypt := "hello world!"
		key := tst.SecretKey()

		enc1 := New(key)
		encryptedMsg := enc1.Encrypt(msgToEncrypt)

		enc2 := New(key) // server restarted
		decryptedMsg, err := enc2.Decrypt(encryptedMsg)
		attest.Ok(t, err)
		attest.Equal(t, string(decryptedMsg), msgToEncrypt)
	})

	t.Run("same key gives same salt", func(t *testing.T) {
		t.Parallel()

		key := tst.SecretKey()
		enc1 := New(key)
		enc2 := New(key)

		attest.Equal(t, enc1.salt, enc2.salt)
		attest.Equal(t, len(enc1.salt), saltLen)
		attest.Equal(t, cap(enc1.salt), saltLen)

		enc3 := New("an0ther-v3ry-h@rd-Pas1word")
		attest.NotEqual(t, enc1.salt, enc3.salt)
	})

	t.Run("different key cannot decrypt", func(t *testing.T) {
		t.Parallel()

		enc1 := New(tst.SecretKey())
		enc2 := New("an0ther-v3ry-h@rd-Pas1word")

		encryptedMsg := enc1.Encrypt("hello world!")
		_, err := enc2.Decrypt(encryptedMsg)
		attest.Error(t, err)
		attest.Subsequence(t, err.Error(), "unknown salt")
	})

	t.Run("unknown salt is rejected without key derivation", func(t *testing.T) {
		t.Parallel()

		enc := New(tst.SecretKey())
		encryptedMsg := enc.Encrypt("hello world!")

		// A key derivation(argon2) takes about 35ms. So 100 of them would take more than 3 seconds.
		start := time.Now()
		for i := range 100 {
			msg := slices.Clone(encryptedMsg)
			msg[0] = byte(i) + 1
			msg[1] ^= 0xff // makes sure that the salt is different, even if msg[0] is the same as before.
			_, err := enc.Decrypt(msg)
			attest.Error(t, err)
			attest.Subsequence(t, err.Error(), "unknown salt")
		}
		attest.True(t, time.Since(start) < 1*time.Second, attest.Sprint(time.Since(start)))
	})

	t.Run("tampered ciphertext is rejected", func(t *testing.T) {
		t.Parallel()

		enc := New(tst.SecretKey())
		encryptedMsg := enc.Encrypt("hello world!")
		encryptedMsg[len(encryptedMsg)-1] ^= 0xff

		_, err := enc.Decrypt(encryptedMsg)
		attest.Error(t, err)
	})

	t.Run("shared Enc is concurrency safe", func(t *testing.T) {
		t.Parallel()

		enc := New(tst.SecretKey())
		wg := &sync.WaitGroup{}
		for rN := range 8 {
			wg.Go(func() {
				msg := strings.Repeat("a", rN)
				decryptedMsg, err := enc.Decrypt(enc.Encrypt(msg))
				attest.Ok(t, err)
				attest.Equal(t, string(decryptedMsg), msg)
			})
		}
		wg.Wait()
	})

	t.Run("encrypt/decrypt file", func(t *testing.T) {
		t.Parallel()

		msgToEncrypt := ""
		var decryptedMsg []byte
		key := tst.SecretKey()

		dir := t.TempDir()
		originalFile := dir + "/originalFile.txt"
		encryptedFile := dir + "/encryptedFile.txt.encrypted"

		{
			err := os.WriteFile(originalFile, []byte(strings.Repeat("h", (50*1024*1024))), 0o666) // 50MB
			attest.Ok(t, err)
		}

		{
			b, err := os.ReadFile(originalFile)
			attest.Ok(t, err)
			msgToEncrypt = string(b)

			enc := New(key)
			er := os.WriteFile(encryptedFile, enc.Encrypt(msgToEncrypt), 0o666)
			attest.Ok(t, er)
		}

		{
			b, err := os.ReadFile(encryptedFile)
			attest.Ok(t, err)

			enc := New(key)
			msg, err := enc.Decrypt(b)
			attest.Ok(t, err)
			decryptedMsg = msg
		}

		attest.Equal(t, string(decryptedMsg), msgToEncrypt)
	})

	t.Run("concurrency safe", func(t *testing.T) {
		t.Parallel()

		msgToEncrypt := "hello world!"

		run := func() {
			key := tst.SecretKey()
			enc := New(key)

			encryptedMsg := enc.Encrypt(msgToEncrypt)
			decryptedMsg, err := enc.Decrypt(encryptedMsg)
			attest.Ok(t, err)
			attest.Equal(t, string(decryptedMsg), msgToEncrypt)
		}

		wg := &sync.WaitGroup{}
		for rN := 0; rN <= 7; rN++ {
			wg.Go(func() {
				run()
			})
		}
		wg.Wait()
	})
}

var result []byte //nolint:gochecknoglobals

func BenchmarkEnc(b *testing.B) {
	var r []byte
	msgToEncrypt := "hello world!"
	key := tst.SecretKey()
	enc := New(key)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		encryptedMsg := enc.Encrypt(msgToEncrypt)
		decryptedMsg, err := enc.Decrypt(encryptedMsg)
		r = decryptedMsg
		attest.Ok(b, err)
		attest.Equal(b, string(decryptedMsg), msgToEncrypt)
	}
	// always store the result to a package level variable
	// so the compiler cannot eliminate the Benchmark itself.
	result = r
}

func BenchmarkDecryptUnknownSalt(b *testing.B) {
	enc := New(tst.SecretKey())
	msg := enc.Encrypt("hello world!")
	msg[0] ^= 0xff // a salt that is not the salt of enc.

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_, _ = enc.Decrypt(msg)
	}
}
