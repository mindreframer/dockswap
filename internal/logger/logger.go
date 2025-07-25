package logger

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

const (
	LevelError = 1
	LevelInfo  = 2
	LevelDebug = 3
)

type Logger interface {
	Error(format string, args ...interface{})
	Info(format string, args ...interface{})
	Debug(format string, args ...interface{})
	SetLevel(level int)
	GetLevel() int
}

type StandardLogger struct {
	level  int
	output io.Writer
	mutex  sync.Mutex
}

func New(level int) Logger {
	if level < LevelError || level > LevelDebug {
		level = LevelInfo
	}
	return &StandardLogger{
		level:  level,
		output: os.Stderr,
	}
}

func NewWithOutput(level int, output io.Writer) Logger {
	if level < LevelError || level > LevelDebug {
		level = LevelInfo
	}
	return &StandardLogger{
		level:  level,
		output: output,
	}
}

func (l *StandardLogger) SetLevel(level int) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if level >= LevelError && level <= LevelDebug {
		l.level = level
	}
}

func (l *StandardLogger) GetLevel() int {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	return l.level
}

func (l *StandardLogger) Error(format string, args ...interface{}) {
	l.log(LevelError, "ERROR", format, args...)
}

func (l *StandardLogger) Info(format string, args ...interface{}) {
	if l.level >= LevelInfo {
		l.log(LevelInfo, "INFO", format, args...)
	}
}

func (l *StandardLogger) Debug(format string, args ...interface{}) {
	if l.level >= LevelDebug {
		l.log(LevelDebug, "DEBUG", format, args...)
	}
}

func (l *StandardLogger) log(messageLevel int, levelName, format string, args ...interface{}) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if l.level < messageLevel {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	message := fmt.Sprintf(format, args...)
	fmt.Fprintf(l.output, "[%s] %s: %s\n", timestamp, levelName, message)
}
