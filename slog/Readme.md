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

    func Debug(format string, v ...interface{})
        Debug will log with a DEBUG prefix if DebugLevel is se

    func Error(format string, v ...interface{})
        Error will log with a ERROR prefix

    func FatalOnErr(err error, format string, v ...interface{})
        FatalOnErr if error provided, will log out details of an error and exi

    func Info(format string, v ...interface{})
        Info formats logs with an INFO prefix

    func Log(format string, v ...interface{})
        Log formats logs directly to the main logger

    func PanicOnErr(err error, format string, v ...interface{})
        PanicOnErr if error provided, will log out details of an error and exi

    func SetFlags(flag int)
        SetFlags allows changing the logger flags using flags found in `log`

    func WarnOnErr(err error, format string, v ...interface{})
        WarnOnErr if error provided, will provide a warning if an error is provided

    func Warning(format string, v ...interface{})
        Warning will log with a WARNING prefix
