// Package debug holds the -debug switch. Debug messages are written as
// "debug" events to the run's log file, never to the terminal.
package debug

import (
	"fmt"

	"excentrico-tools-go/internal/logger"
)

var enabled bool

func SetEnabled(enable bool) {
	enabled = enable
}

// Printf records a debug event when -debug is on.
func Printf(format string, args ...interface{}) {
	if enabled {
		logger.Get().Debug(&logger.WideEvent{Operation: "debug", Message: fmt.Sprintf(format, args...)})
	}
}

func IsEnabled() bool {
	return enabled
}
