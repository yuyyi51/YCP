package utils

import (
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"
	"time"
)

const (
	LogLevelInvalid = iota
	LogLevelTrace
	LogLevelDebug
	LogLevelInfo
	LogLevelNotice
	LogLevelWarn
	LogLevelError
	LogLevelFatal
	LogLevelNone
)

type Logger struct {
	logLevel       int
	skipStackLevel int
}

var DefaultLogger = NewLogger("debug", 2)

func LogLevelToString(level int) string {
	if level <= LogLevelInvalid || level >= LogLevelNone {
		return ""
	}
	switch level {
	case LogLevelTrace:
		return "TRACE"
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelNotice:
		return "NOTICE"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	case LogLevelFatal:
		return "FATAL"
	}
	return ""
}

func StringToLogLevel(level string) int {
	level = strings.ToLower(level)
	switch level {
	case "none":
		return LogLevelNone
	case "trace":
		return LogLevelTrace
	case "debug":
		return LogLevelDebug
	case "info":
		return LogLevelInfo
	case "notice":
		return LogLevelNotice
	case "warn":
		return LogLevelWarn
	case "error":
		return LogLevelError
	case "fatal":
		return LogLevelFatal
	}
	return LogLevelNone
}

func NewLogger(level string, skip int) *Logger {
	return &Logger{
		logLevel:       StringToLogLevel(level),
		skipStackLevel: skip,
	}
}

func (l *Logger) log(format string, level int, args ...interface{}) {
	_, file, line, _ := runtime.Caller(l.skipStackLevel)
	file = path.Base(file)
	format = fmt.Sprintf("%s %s %s:%d %s\n", time.Now().Format(time.RFC3339Nano), LogLevelToString(level), file, line, format)
	fmt.Printf(format, args...)
}

func (l *Logger) Trace(format string, args ...interface{}) {
	if l.logLevel <= LogLevelTrace {
		l.log(format, LogLevelTrace, args...)
	}
}

func (l *Logger) Debug(format string, args ...interface{}) {
	if l.logLevel <= LogLevelDebug {
		l.log(format, LogLevelDebug, args...)
	}
}

func (l *Logger) Info(format string, args ...interface{}) {
	if l.logLevel <= LogLevelInfo {
		l.log(format, LogLevelInfo, args...)
	}
}

func (l *Logger) Notice(format string, args ...interface{}) {
	if l.logLevel <= LogLevelNotice {
		l.log(format, LogLevelNotice, args...)
	}
}

func (l *Logger) Warn(format string, args ...interface{}) {
	if l.logLevel <= LogLevelWarn {
		l.log(format, LogLevelWarn, args...)
	}
}

func (l *Logger) Error(format string, args ...interface{}) {
	if l.logLevel <= LogLevelError {
		l.log(format, LogLevelError, args...)
	}
}

func (l *Logger) Fatal(format string, args ...interface{}) {
	if l.logLevel <= LogLevelFatal {
		l.log(format, LogLevelFatal, args...)
	}
	os.Exit(-1)
}
