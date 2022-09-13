package enc

import (
	"testing"

	"github.com/akshayjshah/attest"
)

func getMultiSecretKeys() (string, string) {
	return "hello world", "its a new day."
}

// func TestMultiEnc(t *testing.T) {
// 	t.Parallel()

// 	t.Run("new", func(t *testing.T) {
// 		t.Parallel()

// 		// okay key
// 		key1, key2 := getMultiSecretKeys()
// 		_ = NewMulti(key1, key2)

// 		// short keys
// 		attest.Panics(t, func() {
// 			_ = NewMulti("hi", key2)
// 		})
// 		attest.Panics(t, func() {
// 			_ = NewMulti(key1, "hi")
// 		})
// 	})

// 	t.Run("encrypt/decrypt", func(t *testing.T) {
// 		t.Parallel()

// 		msgToEncryt := "hello world!"
// 		key1, key2 := getMultiSecretKeys()
// 		enc := NewMulti(key1, key2)

// 		encryptedMsg1, encryptedMsg2 := enc.encrypt(msgToEncryt)

// 		decryptedMsg, err := enc.decrypt(encryptedMsg1, encryptedMsg2)
// 		attest.Ok(t, err)

// 		attest.Equal(t, string(decryptedMsg), msgToEncryt)
// 	})

// 	t.Run("encrypt/decrypt base64", func(t *testing.T) {
// 		t.Parallel()

// 		msgToEncryt := "hello world!"
// 		key1, key2 := getMultiSecretKeys()
// 		enc := NewMulti(key1, key2)

// 		token := enc.EncryptEncode(msgToEncryt)

// 		decryptedMsg, err := enc.DecryptDecode(token)
// 		attest.Ok(t, err)
// 		attest.Equal(t, string(decryptedMsg), msgToEncryt)
// 	})

// 	t.Run("key rotation", func(t *testing.T) {
// 		t.Parallel()

// 		msgToEncryt := "hello world!"
// 		key1 := "okay what are you"
// 		key2 := "kill it with fire"

// 		tEnc := NewTheMulti(key1, key2)

// 		token := tEnc.EncryptEncode(msgToEncryt)

// 		decryptedMsg, err := tEnc.DecryptDecode(token)
// 		attest.Ok(t, err)
// 		attest.Equal(t, string(decryptedMsg), msgToEncryt)

// 		rotatedKey2 := "brand new key2"
// 		tEncX := NewTheMulti(key1, rotatedKey2)
// 		decryptedMsg2, err := tEncX.DecryptDecode(token)
// 		attest.Ok(t, err)
// 		attest.Equal(t, string(decryptedMsg2), msgToEncryt)

// 		// msgToEncryt := "hello world!"
// 		// key1 := "okay what are you"
// 		// key2 := "kill it with fire"
// 		// enc1 := New(key1)
// 		// enc2 := New(key2)

// 		// encryptedMsg1 := enc1.Encrypt(msgToEncryt)
// 		// encryptedMsg2 := enc2.Encrypt(msgToEncryt)

// 		// encoded1 := base64.RawURLEncoding.EncodeToString(encryptedMsg1)
// 		// encoded2 := base64.RawURLEncoding.EncodeToString(encryptedMsg2)

// 		// encryptedEncodedMsg := encoded1 + separator + encoded2
// 		// fmt.Println("\n\t encryptedEncodedMsg: ", encryptedEncodedMsg)
// 		// encoded := strings.Split(encryptedEncodedMsg, separator)

// 		// resEncryptedMsg1, err := base64.RawURLEncoding.DecodeString(encoded[0])
// 		// if err != nil {
// 		// 	panic(err)
// 		// }
// 		// resEncryptedMsg2, err := base64.RawURLEncoding.DecodeString(encoded[1])
// 		// if err != nil {
// 		// 	panic(err)
// 		// }

// 		// fmt.Println("resEncryptedMsg1; ", resEncryptedMsg1)
// 		// fmt.Println("resEncryptedMsg2; ", resEncryptedMsg2)

// 		// resDecryptedMsg1, err := enc1.Decrypt(resEncryptedMsg1)
// 		// attest.Ok(t, err)

// 		// resDecryptedMsg2, err := enc2.Decrypt(resEncryptedMsg2)
// 		// attest.Ok(t, err)

// 		// fmt.Println("resDecryptedMsg1; ", resDecryptedMsg1, string(resDecryptedMsg1))
// 		// fmt.Println("resDecryptedMsg2; ", resDecryptedMsg2, string(resDecryptedMsg2))

// 		// decryptedMsg1, err := enc1.Decrypt(encryptedMsg1)
// 		// attest.Ok(t, err)
// 		// attest.Equal(t, string(decryptedMsg1), msgToEncryt)

// 		// msgToEncryt := "hello world!"
// 		// key1, key2 := getMultiSecretKeys()
// 		// enc := NewMulti(key1, key2)

// 		// token := enc.EncryptEncode(msgToEncryt)

// 		// decryptedMsg, err := enc.DecryptDecode(token)
// 		// attest.Ok(t, err)
// 		// attest.Equal(t, string(decryptedMsg), msgToEncryt)

// 		// rotatedKey2 := "brand new key2"
// 		// encX := NewMulti(key1, rotatedKey2)
// 		// decryptedMsg2, err := encX.DecryptDecode(token)
// 		// // if err != nil {
// 		// // 	debug.PrintStack()
// 		// // 	// panic(err)
// 		// // }
// 		// attest.Ok(t, err)
// 		// attest.Equal(t, string(decryptedMsg2), msgToEncryt)
// 	})

// 	t.Run("concurrency safe", func(t *testing.T) {
// 		t.Parallel()

// 		msgToEncryt := "hello world!"

// 		run := func() {
// 			key1, key2 := getMultiSecretKeys()
// 			enc := NewMulti(key1, key2)

// 			encryptedMsg1, encryptedMsg2 := enc.encrypt(msgToEncryt)
// 			decryptedMsg, err := enc.decrypt(encryptedMsg1, encryptedMsg2)
// 			attest.Ok(t, err)
// 			attest.Equal(t, string(decryptedMsg), msgToEncryt)
// 		}

// 		wg := &sync.WaitGroup{}
// 		for rN := 0; rN <= 7; rN++ {
// 			wg.Add(1)
// 			go func() {
// 				defer wg.Done()
// 				run()
// 			}()
// 		}
// 		wg.Wait()
// 	})
// }

func TestMultiEnc(t *testing.T) {
	t.Parallel()

	t.Run("key rotation", func(t *testing.T) {
		t.Parallel()

		msgToEncryt := "hello world!"
		key1 := "okay what are you"
		key2 := "kill it with fire"

		tEnc := NewTheMulti(key1, key2)

		token := tEnc.EncryptEncode(msgToEncryt)

		decryptedMsg, err := tEnc.DecryptDecode(token)
		attest.Ok(t, err)
		attest.Equal(t, string(decryptedMsg), msgToEncryt)

		rotatedKey2 := "brand new key2"
		tEncX := NewTheMulti(key1, rotatedKey2)
		decryptedMsg2, err := tEncX.DecryptDecode(token)
		attest.Ok(t, err)
		attest.Equal(t, string(decryptedMsg2), msgToEncryt)
	})
}
