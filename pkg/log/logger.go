package log

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
)

func init() {
	log.SetPrefix("")
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func SetFlags(flag int) {
	log.SetPrefix("")
	log.SetFlags(flag)
}

func LogPrefix() string {
	return fmt.Sprintf("%s %s:", time.Now().Format("2006/01/02 15:04:05"), line())
}

func LogPrefixWith(level string) string {
	return fmt.Sprintf("%s %s: %s:", time.Now().Format("2006/01/02 15:04:05"), line(), level)
}

func NewLogger(name string) Logger {
	return map[string]any{
		"method": name,
		"time":   time.Now(),
	}
}

func NewLoggerWith(name string, key string, value any) Logger {
	logger := NewLogger(name)
	logger.Set(key, value)
	return logger
}

type Logger map[string]any

func (ins Logger) Set(key string, value any) {
	ins[key] = value
}

func (l Logger) JsonEncode() string {
	l.Set("line", line())
	json, _ := json.Marshal(l)
	return string(json)
}
