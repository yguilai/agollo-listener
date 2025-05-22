package apollox

import "fmt"

type (
	Logger interface {
		Errorf(format string, args ...interface{})
		Warnf(format string, args ...interface{})
	}

	defaultLogger struct{}
)

var myLogger Logger

func SetLogger(logger Logger) {
	myLogger = logger
}

func getLogger() Logger {
	if myLogger == nil {
		myLogger = &defaultLogger{}
	}
	return myLogger
}

func (l *defaultLogger) Errorf(format string, args ...interface{}) {
	fmt.Printf("[ERROR] "+format, args...)
}

func (l *defaultLogger) Warnf(format string, args ...interface{}) {
	fmt.Printf("[WARN] "+format, args...)
}
