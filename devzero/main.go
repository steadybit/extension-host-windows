package main

import (
	"fmt"
	"os"
	"syscall"
)

func main() {
	if len(os.Args) > 1 {
		fmt.Println("'devzero' utility emulates the Linux /dev/zero and is used as stdin to other processes like 'dd'.")
		fmt.Println("usage: run the application without arguments and pipe its output to another executable.\nexample: devzero | <other_executable> [flags]")
		os.Exit(0)
	}

	fi, err := os.Stdout.Stat()
	if err != nil {
		os.Exit(1)
	}
	if (fi.Mode() & os.ModeCharDevice) != 0 {
		os.Stderr.WriteString("This program is intended to be used in a pipe.\n")
		os.Exit(1)
	}

	zeroBlock := make([]byte, 64*1024)
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
	for {
		if errno, ok := err.(syscall.Errno); ok {
			return errno == syscall.EPIPE
		}
		unwrapped := unwrap(err)
		if unwrapped == nil {
			break
		}
		err = unwrapped
	}
	return false
}

func unwrap(err error) error {
	type unwrapper interface {
		Unwrap() error
	}
	if u, ok := err.(unwrapper); ok {
		return u.Unwrap()
	}
	return nil
}
