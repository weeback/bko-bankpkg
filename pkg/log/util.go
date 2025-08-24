package log

import (
	"fmt"
	"path/filepath"
	"runtime"
)

func line() string {
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		return ""
	}
	if file == "" {
		file = "???"
	} else {
		_, file = filepath.Split(file)
	}
	return fmt.Sprintf("%s:%d", file, line)
}
