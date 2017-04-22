package util

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
)

// FailOnError log stack and fatal with given message
func FailOnError(err error, msg string) {
	if err != nil {
		stack := debug.Stack()
		log.Fatalf("%s: %s - stack:\n%s", msg, err, stack)
	}
}

// WrapError wrap a error with given message
func WrapError(err error, msg string) error {
	if err != nil {
		return fmt.Errorf("%s: %s", msg, err)
	}
	return nil
}

// GetRootPath get current cmd path
func GetRootPath() string {
	p, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		panic(err)
	}
	return p
}
