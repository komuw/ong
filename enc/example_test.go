package enc_test

import (
	"crypto/rand"
	"fmt"

	"github.com/komuw/ong/enc"
)

func ExampleEnc_Encrypt() {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic(err)
	}

	e := enc.New(key)

	plainTextMsg := "Muziki asili yake - Remmy Ongala." // English: `What is the origin of music by Remmy Ongala`
	encryptedMsg := e.Encrypt(plainTextMsg)
	_ = encryptedMsg

	// Output:
}

func ExampleEnc_EncryptEncode() {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic(err)
	}

	e := enc.New(key)

	originalPlainTextMsg := "three little birds."
	encryptedEncodedMsg := e.EncryptEncode(originalPlainTextMsg)

	resultantPlainTextMsg, err := e.DecryptDecode(encryptedEncodedMsg)
	if err != nil {
		panic(err)
	}

	if resultantPlainTextMsg != originalPlainTextMsg {
		panic("something went wrong")
	}

	fmt.Println(resultantPlainTextMsg)

	// Output: three little birds.
}
