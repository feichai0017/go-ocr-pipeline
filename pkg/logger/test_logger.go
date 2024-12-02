package logger

import (
	"sync"
)

// TestLogger 用于测试的日志记录器
type TestLogger struct {
	mu      sync.Mutex
	entries []LogEntry
}

// Named implements Logger.
func (l *TestLogger) Named(name string) Logger {
	panic("unimplemented")
}

// Sync implements Logger.
func (l *TestLogger) Sync() error {
	panic("unimplemented")
}

type LogEntry struct {
	Level   string
	Message string
	Fields  []Field
}

// NewTestLogger 创建一个新的测试日志记录器
func NewTestLogger() *TestLogger {
	return &TestLogger{
		entries: make([]LogEntry, 0),
	}
}

func (l *TestLogger) Debug(msg string, fields ...Field) {
	l.log("DEBUG", msg, fields...)
}

func (l *TestLogger) Info(msg string, fields ...Field) {
	l.log("INFO", msg, fields...)
}

func (l *TestLogger) Warn(msg string, fields ...Field) {
	l.log("WARN", msg, fields...)
}

func (l *TestLogger) Error(msg string, fields ...Field) {
	l.log("ERROR", msg, fields...)
}

func (l *TestLogger) Fatal(msg string, fields ...Field) {
	l.log("FATAL", msg, fields...)
}

func (l *TestLogger) With(fields ...Field) Logger {
	return l
}

func (l *TestLogger) log(level, msg string, fields ...Field) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.entries = append(l.entries, LogEntry{
		Level:   level,
		Message: msg,
		Fields:  fields,
	})
}

// GetEntries 返回所有日志条目
func (l *TestLogger) GetEntries() []LogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()

	entries := make([]LogEntry, len(l.entries))
	copy(entries, l.entries)
	return entries
}

// Clear 清除所有日志条目
func (l *TestLogger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = l.entries[:0]
}
