package logging

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// LogLevel represents the severity of a log message
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l LogLevel) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Logger wraps slog.Logger with additional functionality for gollama-chat
type Logger struct {
	slogger *slog.Logger
	level   LogLevel
	file    *os.File
}

var globalLogger *Logger

// Config represents logging configuration
type Config struct {
	Level        LogLevel
	EnableFile   bool
	LogDir       string // Optional: defaults to standard user log directory
	EnableStderr bool   // Whether to log to stderr (disable for TUI mode)
}

// DefaultConfig returns sensible logging defaults
func DefaultConfig() *Config {
	return &Config{
		Level:        LevelInfo,
		EnableFile:   true,
		LogDir:       DefaultDir(),
		EnableStderr: false, // Default to false for TUI applications
	}
}

// DefaultDir returns the standard location for user-level logs
func DefaultDir() string {
	var logDir string

	switch runtime.GOOS {
	case "linux", "freebsd", "openbsd", "netbsd":
		// XDG Base Directory Specification
		if xdgData := os.Getenv("XDG_DATA_HOME"); xdgData != "" {
			logDir = filepath.Join(xdgData, "gollama-chat", "logs")
		} else {
			home, _ := os.UserHomeDir()
			logDir = filepath.Join(home, ".local", "share", "gollama-chat", "logs")
		}
	case "darwin":
		// macOS standard location
		home, _ := os.UserHomeDir()
		logDir = filepath.Join(home, "Library", "Logs", "gollama-chat")
	case "windows":
		// Windows standard location
		if appData := os.Getenv("LOCALAPPDATA"); appData != "" {
			logDir = filepath.Join(appData, "gollama-chat", "logs")
		} else {
			home, _ := os.UserHomeDir()
			logDir = filepath.Join(home, "AppData", "Local", "gollama-chat", "logs")
		}
	default:
		// Fallback
		home, _ := os.UserHomeDir()
		logDir = filepath.Join(home, ".gollama-chat", "logs")
	}

	return logDir
}

// Initialize sets up the global logger with the given configuration
func Initialize(config *Config) error {
	var writers []io.Writer
	var logFile *os.File

	// Conditionally write to stderr
	if config.EnableStderr {
		writers = append(writers, os.Stderr)
	}

	// Optionally write to file
	if config.EnableFile {
		if err := os.MkdirAll(config.LogDir, 0755); err != nil {
			return fmt.Errorf("failed to create log directory %s: %w", config.LogDir, err)
		}

		// Create log file with timestamp
		timestamp := time.Now().Format("2006-01-02")
		logPath := filepath.Join(config.LogDir, fmt.Sprintf("gollama-chat-%s.log", timestamp))

		var err error
		logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to create log file %s: %w", logPath, err)
		}

		writers = append(writers, logFile)
	}

	// Ensure we have at least one writer
	if len(writers) == 0 {
		// Fallback to stderr if no other writer is configured
		writers = append(writers, os.Stderr)
	}

	// Create multi-writer
	multiWriter := io.MultiWriter(writers...)

	// Convert our log level to slog level
	var slogLevel slog.Level
	switch config.Level {
	case LevelDebug:
		slogLevel = slog.LevelDebug
	case LevelInfo:
		slogLevel = slog.LevelInfo
	case LevelWarn:
		slogLevel = slog.LevelWarn
	case LevelError:
		slogLevel = slog.LevelError
	}

	// Create structured logger with custom handler
	opts := &slog.HandlerOptions{
		Level: slogLevel,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Add caller information for better debugging
			if a.Key == slog.SourceKey {
				if source, ok := a.Value.Any().(*slog.Source); ok {
					// Shorten the file path to just the filename
					source.File = filepath.Base(source.File)
				}
			}
			return a
		},
		AddSource: true,
	}

	handler := slog.NewTextHandler(multiWriter, opts)
	slogger := slog.New(handler)

	globalLogger = &Logger{
		slogger: slogger,
		level:   config.Level,
		file:    logFile,
	}

	// Also set the standard library logger to use our handler for compatibility
	log.SetOutput(multiWriter)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	return nil
}

// GetLogger returns the global logger instance
func GetLogger() *Logger {
	if globalLogger == nil {
		// Initialize with defaults if not already done
		config := DefaultConfig()
		if err := Initialize(config); err != nil {
			// Fallback to stderr-only logging
			globalLogger = &Logger{
				slogger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})),
				level:   LevelInfo,
			}
		}
	}
	return globalLogger
}

// Close closes the log file if it was opened
func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, args ...any) {
	if l.level <= LevelDebug {
		l.slogger.Debug(msg, args...)
	}
}

// Info logs an info message
func (l *Logger) Info(msg string, args ...any) {
	if l.level <= LevelInfo {
		l.slogger.Info(msg, args...)
	}
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, args ...any) {
	if l.level <= LevelWarn {
		l.slogger.Warn(msg, args...)
	}
}

// Error logs an error message
func (l *Logger) Error(msg string, args ...any) {
	if l.level <= LevelError {
		l.slogger.Error(msg, args...)
	}
}

// With returns a new logger with the given attributes
func (l *Logger) With(args ...any) *Logger {
	return &Logger{
		slogger: l.slogger.With(args...),
		level:   l.level,
		file:    l.file,
	}
}

// WithComponent returns a logger with a component attribute
func (l *Logger) WithComponent(component string) *Logger {
	return l.With("component", component)
}

// Convenience functions for global logger

// Debug logs a debug message using the global logger
func Debug(msg string, args ...any) {
	GetLogger().Debug(msg, args...)
}

// Info logs an info message using the global logger
func Info(msg string, args ...any) {
	GetLogger().Info(msg, args...)
}

// Warn logs a warning message using the global logger
func Warn(msg string, args ...any) {
	GetLogger().Warn(msg, args...)
}

// Error logs an error message using the global logger
func Error(msg string, args ...any) {
	GetLogger().Error(msg, args...)
}

// WithComponent returns a logger with a component attribute using the global logger
func WithComponent(component string) *Logger {
	return GetLogger().WithComponent(component)
}

// UpdateLevel updates the logging level for the global logger at runtime
func UpdateLevel(newLevel LogLevel) {
	if globalLogger != nil {
		globalLogger.level = newLevel
		// Note: We cannot update slog handler level at runtime without recreating it
		// For now, we'll update our internal level which is checked in our logging methods
		// In a future improvement, we could reinitialize the handler completely
	}
}

// Reconfigure reinitializes the global logger with a new configuration
// This allows changing log levels and other settings at runtime
func Reconfigure(config *Config) error {
	// Close the existing logger first
	if globalLogger != nil {
		if err := globalLogger.Close(); err != nil {
			// Log the error but don't fail the reconfiguration
			log.Printf("Warning: Failed to close existing logger: %v", err)
		}
	}

	// Reinitialize with the new configuration
	return Initialize(config)
}

// Close closes the global logger
func Close() error {
	if globalLogger != nil {
		return globalLogger.Close()
	}
	return nil
}
