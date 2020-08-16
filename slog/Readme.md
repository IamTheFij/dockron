# slog

A super simple go logger

I know there are many go loggers out there that offer various logging features such as file rotation, granular verbosity settings, colored and JSON output, etc.

_Slog is not one of them._

Slog lets you hide or show debug logs as well as provides a simpler way to log messages with Warning and Error prefixes for consistency.

Also provided are a few simple methods for handling returned `error` variables, logging them out and optionally panicing or fatally exiting.

## Documentation
    package slog // import "github.com/iamthefij/dockron/slog"

    Package slog is a super simple logger that allows a few convenience methods
    for handling debug vs warning/error logs. It also adds a few conveniences
    for handling errors.

    VARIABLES

    var (
    	// DebugLevel indicates if we should log at the debug level
    	DebugLevel = true
    )

    FUNCTIONS

    func FatalErr(err error, format string, v ...interface{})
        FatalErr if error provided, will log out details of an error and exi

    func Log(format string, v ...interface{})
        Log formats logs directly to the main logger

    func LogDebug(format string, v ...interface{})
        LogDebug will log with a DEBUG prefix if DebugLevel is se

    func LogError(format string, v ...interface{})
        LogError will log with a ERROR prefix

    func LogWarning(format string, v ...interface{})
        LogWarning will log with a WARNING prefix

    func PanicErr(err error, format string, v ...interface{})
        PanicErr if error provided, will log out details of an error and exi

    func WarnErr(err error, format string, v ...interface{})
        WarnErr if error provided, will provide a warning if an error is provided
