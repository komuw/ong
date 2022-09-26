package cry_test

import (
	"fmt"

	"github.com/komuw/ong/cry"
)

func ExampleEnc_Encrypt() {
	key := "hard-passwd"
	e := cry.New(key)

	plainTextMsg := "Muziki asili yake - Remmy Ongala." // English: `What is the origin of music by Remmy Ongala`
	encryptedMsg := e.Encrypt(plainTextMsg)
	_ = encryptedMsg

	// Output:
}

func ExampleEnc_EncryptEncode() {
	key := "hard-passwd"
	e := cry.New(key)

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