package server

import (
	"os"

	"golang.org/x/sys/unix" // syscall package is deprecated
)

// setRlimit tries to set the RLIMIT_NOFILE to a higher value.
// Since the proposal in [3] has been implemented this func can be removed after Go1.19
//
// 1. http://0pointer.net/blog/file-descriptor-limits.html
// 2. https://github.com/systemd/systemd/blob/e7901aba1480db21e06e21cef4f6486ad71b2ec5/src/basic/rlimit-util.c#L373
// 3. https://github.com/golang/go/issues/46279
// 4. https://linux.die.net/man/2/setrlimit
func setRlimit() {
	var err error
	defer func() {
		if err != nil && os.Getenv("GOWEB_RUNNING_IN_TESTS") != "" {
			panic(err)
		}
	}()

	// unix.Rlimit.Cur is the soft limit: value that the kernel enforces for the corresponding resource.
	// unix.Rlimit.Max is the hard limit: ceiling for the soft limit.
	// An unprivileged process may only set its soft limit to a value in the range from 0 upto hard-limit.
	//
	// - https://linux.die.net/man/2/setrlimit

	var targetRlimit uint64 = 512_000 // value taken from link 1 above.
	var currentRlimit unix.Rlimit
	var newRlimit unix.Rlimit

	err = unix.Getrlimit(unix.RLIMIT_NOFILE, &currentRlimit)

	newRlimit.Cur = currentRlimit.Cur
	newRlimit.Max = currentRlimit.Max

	if newRlimit.Cur < targetRlimit {
		newRlimit.Cur = targetRlimit
	}
	newRlimit.Max = newRlimit.Cur + 256 // set hard-limit higher than soft-limit.

	err = unix.Setrlimit(unix.RLIMIT_NOFILE, &newRlimit)
}
