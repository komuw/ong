package automax_test

import (
	"github.com/komuw/ong/automax"
)

func ExampleSetMem() {
	undo := automax.SetMem()
	defer undo()
}

func ExampleSetCpu() {
	undo := automax.SetCpu()
	defer undo()
}
