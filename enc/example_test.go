package enc_test

import (
	"fmt"

	"github.com/komuw/ong/enc"
)

func ExampleEnc_Encrypt() {
	key := "hard-passwd"
	e := enc.New(key)

	plainTextMsg := "Muziki asili yake - Remmy Ongala." // English: `What is the origin of music by Remmy Ongala`
	encryptedMsg := e.Encrypt(plainTextMsg)
	_ = encryptedMsg

	// Output:
}

func ExampleEnc_EncryptEncode() {
	key := "hard-passwd"
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

func ExampleMultiEnc_EncryptEncode() {
	key1 := "this is it"
	key2 := "what are we?"
	e := enc.NewMulti(key1, key2)

	originalPlainTextMsg := "three little birds."
	encryptedEncodedMsg := e.EncryptEncode(originalPlainTextMsg)

	resultantPlainTextMsg, err := e.DecryptDecode(encryptedEncodedMsg)
	if err != nil {
		panic(err)
	}

	if resultantPlainTextMsg != originalPlainTextMsg {
		panic("something went wrong")
	}

	// let's say that key2 is compromised and we need to rotate it.
	rotatedKey2 := "brand new key2"
	e2 := enc.NewMulti(key1, rotatedKey2)
	resultantPlainTextMsg2, err := e2.DecryptDecode(encryptedEncodedMsg)
	if err != nil {
		panic(err)
	}

	if resultantPlainTextMsg2 != originalPlainTextMsg {
		panic("something went wrong")
	}

	fmt.Println(resultantPlainTextMsg2)

	// Output: three little birds.
}
