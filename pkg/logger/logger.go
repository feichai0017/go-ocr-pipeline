package logger

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "time"
    
    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
    "gopkg.in/natefinch/lumberjack.v2"
)

// Field type
type Field = zapcore.Field

// Level type
type Level = zapcore.Level

const (
    // DebugLevel level
    DebugLevel Level = zapcore.DebugLevel
    // InfoLevel level
    InfoLevel Level = zapcore.InfoLevel
    // WarnLevel level
    WarnLevel Level = zapcore.WarnLevel
    // ErrorLevel level
    ErrorLevel Level = zapcore.ErrorLevel
    // FatalLevel level
    FatalLevel Level = zapcore.FatalLevel
)

// Logger interface
type Logger interface {
    Debug(msg string, fields ...Field)
    Info(msg string, fields ...Field)
    Warn(msg string, fields ...Field)
    Error(msg string, fields ...Field)
    Fatal(msg string, fields ...Field)
    With(fields ...Field) Logger
    Named(name string) Logger
    Sync() error
}

// Config defines logger configuration
type Config struct {
    Level        string `json:"level" yaml:"level"`
    Encoding     string `json:"encoding" yaml:"encoding"`
    OutputPaths  []string `json:"outputPaths" yaml:"outputPaths"`
    ErrorPaths   []string `json:"errorPaths" yaml:"errorPaths"`
    MaxSize      int      `json:"maxSize" yaml:"maxSize"`         // MB
    MaxBackups   int      `json:"maxBackups" yaml:"maxBackups"`
    MaxAge       int      `json:"maxAge" yaml:"maxAge"`           // days
    Compress     bool     `json:"compress" yaml:"compress"`
    Development  bool     `json:"development" yaml:"development"`
    InitialFields map[string]interface{} `json:"initialFields" yaml:"initialFields"`
}

type logger struct {
    zap *zap.Logger
}

// Option defines logger option function
type Option func(*Config)

// WithLevel sets logger level
func WithLevel(level string) Option {
    return func(c *Config) {
        c.Level = level
    }
}

// WithEncoding sets logger encoding
func WithEncoding(encoding string) Option {
    return func(c *Config) {
        c.Encoding = encoding
    }
}

// WithOutputPaths sets logger output paths
func WithOutputPaths(paths []string) Option {
    return func(c *Config) {
        c.OutputPaths = paths
    }
}

// NewLogger creates a new logger instance
func NewLogger(opts ...Option) (Logger, error) {
    // Default config
    cfg := &Config{
        Level:      "info",
        Encoding:   "json",
        OutputPaths: []string{"stdout", "logs/app.log"},
        ErrorPaths:  []string{"stderr", "logs/error.log"},
        MaxSize:    100,
        MaxBackups: 3,
        MaxAge:     7,
        Compress:   true,
        Development: false,
        InitialFields: make(map[string]interface{}),
    }

    // Apply options
    for _, opt := range opts {
        opt(cfg)
    }

    // Create directories if needed
    for _, path := range append(cfg.OutputPaths, cfg.ErrorPaths...) {
        if path != "stdout" && path != "stderr" {
            if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
                return nil, fmt.Errorf("can't create log directory: %w", err)
            }
        }
    }

    // Create core encoder
    encoderConfig := zapcore.EncoderConfig{
        TimeKey:        "timestamp",
        LevelKey:       "level",
        NameKey:        "logger",
        CallerKey:      "caller",
        FunctionKey:    zapcore.OmitKey,
        MessageKey:     "message",
        StacktraceKey:  "stacktrace",
        LineEnding:     zapcore.DefaultLineEnding,
        EncodeLevel:    zapcore.LowercaseLevelEncoder,
        EncodeTime:     zapcore.ISO8601TimeEncoder,
        EncodeDuration: zapcore.SecondsDurationEncoder,
        EncodeCaller:   zapcore.ShortCallerEncoder,
    }

    // Parse log level
    level := zap.NewAtomicLevel()
    if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
        return nil, fmt.Errorf("can't parse log level: %w", err)
    }

    // Create writers for each output path
    var cores []zapcore.Core
    for _, path := range cfg.OutputPaths {
        var writer zapcore.WriteSyncer
        if path == "stdout" {
            writer = zapcore.AddSync(os.Stdout)
        } else if path == "stderr" {
            writer = zapcore.AddSync(os.Stderr)
        } else {
            writer = zapcore.AddSync(&lumberjack.Logger{
                Filename:   path,
                MaxSize:    cfg.MaxSize,
                MaxBackups: cfg.MaxBackups,
                MaxAge:     cfg.MaxAge,
                Compress:   cfg.Compress,
            })
        }

        var encoder zapcore.Encoder
        if cfg.Encoding == "json" {
            encoder = zapcore.NewJSONEncoder(encoderConfig)
        } else {
            encoder = zapcore.NewConsoleEncoder(encoderConfig)
        }

        cores = append(cores, zapcore.NewCore(
            encoder,
            writer,
            level,
        ))
    }

    // Create options
    options := []zap.Option{
        zap.AddCaller(),
        zap.AddCallerSkip(1),
    }

    if cfg.Development {
        options = append(options, zap.Development())
    }

    // Add initial fields
    if len(cfg.InitialFields) > 0 {
        fields := make([]zap.Field, 0, len(cfg.InitialFields))
        for k, v := range cfg.InitialFields {
            fields = append(fields, zap.Any(k, v))
        }
        options = append(options, zap.Fields(fields...))
    }

    // Create logger
    zapLogger := zap.New(
        zapcore.NewTee(cores...),
        options...,
    )

    return &logger{zap: zapLogger}, nil
}

// Various field constructors
func String(key string, val string) Field        { return zap.String(key, val) }
func Int(key string, val int) Field             { return zap.Int(key, val) }
func Int64(key string, val int64) Field         { return zap.Int64(key, val) }
func Float64(key string, val float64) Field     { return zap.Float64(key, val) }
func Bool(key string, val bool) Field           { return zap.Bool(key, val) }
func Any(key string, val interface{}) Field     { return zap.Any(key, val) }
func Error(err error) Field                     { return zap.Error(err) }
func Time(key string, val time.Time) Field      { return zap.Time(key, val) }
func Duration(key string, val time.Duration) Field { return zap.Duration(key, val) }
func Stack() Field                              { return zap.Stack("stacktrace") }

// Logger implementation
func (l *logger) Debug(msg string, fields ...Field) {
    l.zap.Debug(msg, fields...)
}

func (l *logger) Info(msg string, fields ...Field) {
    l.zap.Info(msg, fields...)
}

func (l *logger) Warn(msg string, fields ...Field) {
    l.zap.Warn(msg, fields...)
}

func (l *logger) Error(msg string, fields ...Field) {
    l.zap.Error(msg, fields...)
}

func (l *logger) Fatal(msg string, fields ...Field) {
    l.zap.Fatal(msg, fields...)
}

func (l *logger) With(fields ...Field) Logger {
    return &logger{zap: l.zap.With(fields...)}
}

func (l *logger) Named(name string) Logger {
    return &logger{zap: l.zap.Named(name)}
}

func (l *logger) Sync() error {
    return l.zap.Sync()
}

// ContextLogger adds context support
type ContextLogger interface {
    Logger
    FromContext(ctx context.Context) Logger
}

type contextLogger struct {
    Logger
}

// NewContextLogger creates a new context logger
func NewContextLogger(l Logger) ContextLogger {
    return &contextLogger{Logger: l}
}

// FromContext creates a new logger with context values
func (l *contextLogger) FromContext(ctx context.Context) Logger {
    // Add context values as fields
    fields := make([]Field, 0)
    if requestID, ok := ctx.Value("request_id").(string); ok {
        fields = append(fields, String("request_id", requestID))
    }
    if userID, ok := ctx.Value("user_id").(string); ok {
        fields = append(fields, String("user_id", userID))
    }
    // Add more context values as needed

    return l.With(fields...)
}