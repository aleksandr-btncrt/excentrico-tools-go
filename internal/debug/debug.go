package debug

import "log"

var enabled bool

func SetEnabled(enable bool) {
	enabled = enable
}

func Printf(format string, args ...interface{}) {
	if enabled {
		log.Printf("DEBUG: "+format, args...)
	}
}

func IsEnabled() bool {
	return enabled
}
