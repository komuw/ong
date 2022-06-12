package server

import (
	"log"

	"golang.org/x/sys/unix" // syscall package is deprecated
)

// setRlimit sets the RLIMIT_NOFILE to a higher value.
// It return a function that you can call to reset the limits to their previous default values.
// proposal in [3] is accepted & implemented; remove this func in Go1.19
//
// 1. http://0pointer.net/blog/file-descriptor-limits.html
// 2. https://github.com/systemd/systemd/blob/e7901aba1480db21e06e21cef4f6486ad71b2ec5/src/basic/rlimit-util.c#L373
// 3. https://github.com/golang/go/issues/46279
func setRlimit() {
	var err error
	defer func() {
		if err != nil {
			log.Println("setRlimit err: ", err)
			panic(err)
		}
	}()

	var targetRlimit uint64 = 512_000 // value taken from link 1 above.
	var currentRlimit unix.Rlimit
	var newRlimit unix.Rlimit

	err = unix.Getrlimit(unix.RLIMIT_NOFILE, &currentRlimit)

	newRlimit.Cur = currentRlimit.Cur
	newRlimit.Max = currentRlimit.Max

	if newRlimit.Max < targetRlimit {
		newRlimit.Max = targetRlimit
	}
	newRlimit.Cur = newRlimit.Max

	err = unix.Setrlimit(unix.RLIMIT_NOFILE, &newRlimit)
}
