package id_test

import (
	"fmt"

	"github.com/komuw/ong/id"
)

func ExampleNew() {
	fmt.Println(id.New())
}

func ExampleRandom() {
	size := 34
	s := id.Random(size)
	if len(s) != size {
		panic("mismatched sizes")
	}
	fmt.Println(s)
}
