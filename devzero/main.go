package main

import (
	"os"
	"syscall"
)

func main() {
	zeroBlock := make([]byte, 4096)

	for {
		_, err := os.Stdout.Write(zeroBlock)
		if err != nil {
			if isBrokenPipe(err) {
				os.Exit(0)
			}
			os.Exit(1)
		}
	}
}

func isBrokenPipe(err error) bool {
	if errno, ok := err.(syscall.Errno); ok {
		return errno == syscall.EPIPE
	}
	return false
}
