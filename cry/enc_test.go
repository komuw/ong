package cry

import (
	"os"
	"slices"
	"strings"
	"sync"
	"testing"

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

	t.Run("encrypt same msg is unique", func(t *testing.T) {
		t.Parallel()

		// This is a useful property especially in how we use it in csrf protection
		// against breachattack.

		msgToEncrypt := "hello world!"
		key := tst.SecretKey()
		enc := New(key)

		encryptedMsg := enc.Encrypt(msgToEncrypt)

		var em []byte
		for i := 0; i < 4; i++ {
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

	t.Run("encrypt/decrypt file", func(t *testing.T) {
		t.Parallel()

		msgToEncrypt := ""
		decryptedMsg := []byte{}
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
			wg.Add(1)
			go func() {
				defer wg.Done()
				run()
			}()
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
