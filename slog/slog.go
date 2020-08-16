// Package slog is a super simple logger that allows a few convenience methods
// for handling debug vs warning/error logs. It also adds a few conveniences for
// handling errors.
package slog

import (
	"log"
	"os"
)

var (
	// DebugLevel indicates if we should log at the debug level
	DebugLevel = true

	// Loggers for various levels
	loggerDebug   = log.New(os.Stderr, "DEBUG", log.LstdFlags)
	loggerWarning = log.New(os.Stderr, "WARNING", log.LstdFlags)
	loggerError   = log.New(os.Stderr, "ERROR", log.LstdFlags)
)

// Log formats logs directly to the main logger
func Log(format string, v ...interface{}) {
	log.Printf(format, v...)
}

// LogDebug will log with a DEBUG prefix if DebugLevel is set
func LogDebug(format string, v ...interface{}) {
	if !DebugLevel {
		return
	}
	loggerDebug.Printf(format, v...)
}

// LogWarning will log with a WARNING prefix
func LogWarning(format string, v ...interface{}) {
	loggerWarning.Printf(format, v...)
}

// LogError will log with a ERROR prefix
func LogError(format string, v ...interface{}) {
	loggerError.Printf(format, v...)
}

// WarnErr if error provided, will provide a warning if an error is provided
func WarnErr(err error, format string, v ...interface{}) {
	if err != nil {
		loggerWarning.Printf(format, v...)
		loggerError.Print(err)
	}
}

// FatalErr if error provided, will log out details of an error and exit
func FatalErr(err error, format string, v ...interface{}) {
	if err != nil {
		loggerError.Printf(format, v...)
		loggerError.Fatal(err)
	}
}

// PanicErr if error provided, will log out details of an error and exit
func PanicErr(err error, format string, v ...interface{}) {
	if err != nil {
		loggerError.Printf(format, v...)
		loggerError.Panic(err)
	}
}
