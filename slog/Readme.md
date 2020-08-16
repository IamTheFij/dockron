# slog

A super simple go logger

I know there are many go loggers out there that offer various logging features such as file rotation, granular verbosity settings, colored and JSON output, etc.

_Slog is not one of them._

Slog lets you hide or show debug logs as well as provides a simpler way to log messages with Warning and Error prefixes for consistency.

Also provided are a few simple methods for handling returned `error` variables, logging them out and optionally panicing or fatally exiting.
