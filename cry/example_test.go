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

func ExampleHash() {
	password := "my NSA-hard password"
	hashedPasswd, err := cry.Hash(password) // save hashedPasswd to the database.
	if err != nil {
		panic(err)
	}

	err = cry.Eql(password, hashedPasswd) // retrieve hashedPasswd from database.
	if err != nil {
		panic(err)
	}

	fmt.Println(hashedPasswd)
}
