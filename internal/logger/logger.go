package logger

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// WideEvent represents a comprehensive log event with all context
type WideEvent struct {
	// Core fields
	Timestamp  string                 `json:"timestamp"`
	Level      string                 `json:"level"` // info, warn, error, debug
	Message    string                 `json:"message"`
	Service    string                 `json:"service,omitempty"`
	Version    string                 `json:"version,omitempty"`
	RequestID  string                 `json:"request_id,omitempty"`
	Operation  string                 `json:"operation,omitempty"` // e.g., "process_film", "upload_media", "download_drive"
	
	// Performance
	DurationMs int64                  `json:"duration_ms,omitempty"`
	
	// Outcome
	Outcome    string                 `json:"outcome,omitempty"` // success, error, partial
	StatusCode int                    `json:"status_code,omitempty"`
	
	// Film context (high cardinality)
	FilmID     string                 `json:"film_id,omitempty"`
	FilmName   string                 `json:"film_name,omitempty"`
	FilmYear   string                 `json:"film_year,omitempty"`
	FilmSection string                `json:"film_section,omitempty"`
	
	// WordPress context
	WordPressPostID int               `json:"wordpress_post_id,omitempty"`
	WordPressMediaID int              `json:"wordpress_media_id,omitempty"`
	WordPressSlug   string            `json:"wordpress_slug,omitempty"`
	
	// Drive context
	DriveFolderID   string            `json:"drive_folder_id,omitempty"`
	DriveFileID     string            `json:"drive_file_id,omitempty"`
	DriveFileName   string            `json:"drive_file_name,omitempty"`
	
	// Image processing context
	ImageOriginalPath string           `json:"image_original_path,omitempty"`
	ImageOptimizedPath string          `json:"image_optimized_path,omitempty"`
	ImageOriginalSize int64            `json:"image_original_size,omitempty"`
	ImageOptimizedSize int64           `json:"image_optimized_size,omitempty"`
	ImageReductionPercent float64      `json:"image_reduction_percent,omitempty"`
	
	// Counts and metrics
	TotalFiles      int                `json:"total_files,omitempty"`
	ImageFiles      int                `json:"image_files,omitempty"`
	FilesDownloaded int                `json:"files_downloaded,omitempty"`
	FilesSkipped    int                `json:"files_skipped,omitempty"`
	FilesUploaded   int                `json:"files_uploaded,omitempty"`
	ImagesProcessed int                `json:"images_processed,omitempty"`
	
	// Error context
	Error           *ErrorContext      `json:"error,omitempty"`
	
	// Additional context (flexible)
	Context         map[string]any    `json:"context,omitempty"`
}

// ErrorContext provides detailed error information
type ErrorContext struct {
	Type      string `json:"type,omitempty"`
	Code      string `json:"code,omitempty"`
	Message   string `json:"message,omitempty"`
	Retriable bool   `json:"retriable,omitempty"`
	Stack     string `json:"stack,omitempty"`
}

// Logger handles structured logging with wide events
type Logger struct {
	service     string
	version     string
	sampleRate  float64 // 0.0 to 1.0, percentage of successful events to log
	errorRate   float64 // Always log errors (1.0)
	slowThresholdMs int64 // Always log operations slower than this
	logFile     *os.File // File handle for log output
	logFilePath string   // Path to the log file
	mu          sync.Mutex // Mutex for thread-safe file writes
}

// NewLogger creates a new logger instance
func NewLogger(service string) *Logger {
	l := &Logger{
		service:         service,
		sampleRate:      0.1, // Sample 10% of successful operations by default
		errorRate:       1.0, // Always log errors
		slowThresholdMs: 5000, // Log operations slower than 5 seconds
	}
	
	// Create unique log file for this run
	l.initLogFile()
	
	return l
}

// initLogFile creates a unique log file for this run
func (l *Logger) initLogFile() {
	// Create logs directory if it doesn't exist
	logsDir := "logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		log.Printf("WARNING: Failed to create logs directory: %v", err)
		return
	}
	
	// Generate unique filename using timestamp
	now := time.Now()
	timestamp := now.Format("2006-01-02T15-04-05")
	// Add nanoseconds to ensure uniqueness even if called multiple times in the same second
	nanos := now.Nanosecond()
	logFileName := fmt.Sprintf("run-%s-%09d.log", timestamp, nanos)
	logFilePath := filepath.Join(logsDir, logFileName)
	
	// Open file in append mode (creates if doesn't exist)
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("WARNING: Failed to open log file %s: %v", logFilePath, err)
		return
	}
	
	l.logFile = logFile
	l.logFilePath = logFilePath
	
	// Write initial log entry
	fmt.Fprintf(logFile, "=== Log run started at %s ===\n", now.Format(time.RFC3339Nano))
}

// SetSampleRate sets the sampling rate for successful operations (0.0 to 1.0)
func (l *Logger) SetSampleRate(rate float64) {
	if rate < 0 {
		rate = 0
	}
	if rate > 1 {
		rate = 1
	}
	l.sampleRate = rate
}

// SetSlowThreshold sets the threshold in milliseconds for always logging slow operations
func (l *Logger) SetSlowThreshold(ms int64) {
	l.slowThresholdMs = ms
}

// GetLogFilePath returns the path to the current log file
func (l *Logger) GetLogFilePath() string {
	return l.logFilePath
}

// Close closes the log file
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	if l.logFile != nil {
		// Write closing entry
		fmt.Fprintf(l.logFile, "=== Log run ended at %s ===\n", time.Now().Format(time.RFC3339Nano))
		err := l.logFile.Close()
		l.logFile = nil
		return err
	}
	return nil
}

// shouldSample determines if an event should be logged based on tail sampling rules
func (l *Logger) shouldSample(event *WideEvent) bool {
	// Always keep errors
	if event.Level == "error" || event.Error != nil {
		return true
	}
	
	// Always keep slow operations
	if event.DurationMs > 0 && event.DurationMs > l.slowThresholdMs {
		return true
	}
	
	// Always keep warnings
	if event.Level == "warn" {
		return true
	}
	
	// For successful operations, use sampling
	if event.Outcome == "success" {
		// Simple random sampling (in production, use better randomness)
		// For now, we'll use a simple hash-based approach
		return l.randomSample()
	}
	
	// Default: log it
	return true
}

// randomSample returns true based on the sample rate
// Uses a simple hash-based approach on timestamp for consistent sampling
func (l *Logger) randomSample() bool {
	if l.sampleRate >= 1.0 {
		return true
	}
	if l.sampleRate <= 0.0 {
		return false
	}
	// Use nanosecond timestamp for sampling (good enough for logging purposes)
	// This provides pseudo-random distribution without requiring crypto/rand
	now := time.Now().UnixNano()
	// Use modulo 1000 for finer granularity
	return (now % 1000) < int64(l.sampleRate*1000)
}

// emit logs the event if it passes sampling
func (l *Logger) emit(event *WideEvent) {
	if !l.shouldSample(event) {
		return
	}
	
	// Set default service and version if not set
	if event.Service == "" {
		event.Service = l.service
	}
	if event.Version == "" {
		event.Version = l.version
	}
	
	// Ensure timestamp is set
	if event.Timestamp == "" {
		event.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	
	// Marshal to JSON
	jsonBytes, err := json.Marshal(event)
	if err != nil {
		// Fallback to simple log if JSON marshaling fails
		log.Printf("LOGGER ERROR: Failed to marshal event: %v", err)
		return
	}
	
	// Output as single line JSON (structured logging)
	jsonLine := string(jsonBytes) + "\n"
	fmt.Fprint(os.Stdout, jsonLine)
	
	// Also write to log file if available
	l.mu.Lock()
	if l.logFile != nil {
		_, writeErr := l.logFile.WriteString(jsonLine)
		if writeErr != nil {
			// Don't log to avoid recursion, but we could use stderr
			fmt.Fprintf(os.Stderr, "LOGGER ERROR: Failed to write to log file: %v\n", writeErr)
		} else {
			// Flush to ensure logs are written immediately
			l.logFile.Sync()
		}
	}
	l.mu.Unlock()
}

// Info logs an informational event
func (l *Logger) Info(event *WideEvent) {
	event.Level = "info"
	if event.Outcome == "" {
		event.Outcome = "success"
	}
	l.emit(event)
}

// Warn logs a warning event
func (l *Logger) Warn(event *WideEvent) {
	event.Level = "warn"
	l.emit(event)
}

// Error logs an error event
func (l *Logger) Error(event *WideEvent) {
	event.Level = "error"
	event.Outcome = "error"
	l.emit(event)
}

// Debug logs a debug event (always logged, not sampled)
func (l *Logger) Debug(event *WideEvent) {
	event.Level = "debug"
	l.emit(event)
}

// StartOperation creates a new event and starts timing
func (l *Logger) StartOperation(operation string) *OperationTracker {
	return &OperationTracker{
		logger:    l,
		event: &WideEvent{
			Operation: operation,
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		},
		startTime: time.Now(),
	}
}

// OperationTracker tracks an operation and emits a wide event when completed
type OperationTracker struct {
	logger    *Logger
	event     *WideEvent
	startTime time.Time
}

// WithFilm adds film context to the event
func (ot *OperationTracker) WithFilm(filmID, filmName, year, section string) *OperationTracker {
	ot.event.FilmID = filmID
	ot.event.FilmName = filmName
	ot.event.FilmYear = year
	ot.event.FilmSection = section
	return ot
}

// WithWordPress adds WordPress context
func (ot *OperationTracker) WithWordPress(postID, mediaID int, slug string) *OperationTracker {
	ot.event.WordPressPostID = postID
	ot.event.WordPressMediaID = mediaID
	ot.event.WordPressSlug = slug
	return ot
}

// WithDrive adds Drive context
func (ot *OperationTracker) WithDrive(folderID, fileID, fileName string) *OperationTracker {
	ot.event.DriveFolderID = folderID
	ot.event.DriveFileID = fileID
	ot.event.DriveFileName = fileName
	return ot
}

// WithImage adds image processing context
func (ot *OperationTracker) WithImage(originalPath, optimizedPath string, originalSize, optimizedSize int64) *OperationTracker {
	ot.event.ImageOriginalPath = originalPath
	ot.event.ImageOptimizedPath = optimizedPath
	ot.event.ImageOriginalSize = originalSize
	ot.event.ImageOptimizedSize = optimizedSize
	if originalSize > 0 {
		ot.event.ImageReductionPercent = float64(originalSize-optimizedSize) / float64(originalSize) * 100
	}
	return ot
}

// WithCounts adds count metrics
func (ot *OperationTracker) WithCounts(totalFiles, imageFiles, downloaded, skipped, uploaded, processed int) *OperationTracker {
	ot.event.TotalFiles = totalFiles
	ot.event.ImageFiles = imageFiles
	ot.event.FilesDownloaded = downloaded
	ot.event.FilesSkipped = skipped
	ot.event.FilesUploaded = uploaded
	ot.event.ImagesProcessed = processed
	return ot
}

// WithError adds error context
func (ot *OperationTracker) WithError(err error) *OperationTracker {
	if err != nil {
		ot.event.Error = &ErrorContext{
			Type:    fmt.Sprintf("%T", err),
			Message: err.Error(),
		}
		ot.event.Outcome = "error"
	}
	return ot
}

// WithContext adds arbitrary context
func (ot *OperationTracker) WithContext(key string, value any) *OperationTracker {
	if ot.event.Context == nil {
		ot.event.Context = make(map[string]any)
	}
	ot.event.Context[key] = value
	return ot
}

// Complete finishes the operation and logs the event
func (ot *OperationTracker) Complete(message string) {
	ot.event.DurationMs = time.Since(ot.startTime).Milliseconds()
	ot.event.Message = message
	if ot.event.Outcome == "" {
		ot.event.Outcome = "success"
	}
	ot.logger.Info(ot.event)
}

// Fail finishes the operation with an error
func (ot *OperationTracker) Fail(message string, err error) {
	ot.event.DurationMs = time.Since(ot.startTime).Milliseconds()
	ot.event.Message = message
	ot.WithError(err)
	ot.logger.Error(ot.event)
}

// Warn logs a warning event during the operation
func (ot *OperationTracker) Warn(event *WideEvent) {
	ot.event.DurationMs = time.Since(ot.startTime).Milliseconds()
	// Merge the passed event into the tracker's event
	if event.Message != "" {
		ot.event.Message = event.Message
	}
	if event.Error != nil {
		ot.event.Error = event.Error
	}
	if event.Context != nil {
		if ot.event.Context == nil {
			ot.event.Context = make(map[string]any)
		}
		for k, v := range event.Context {
			ot.event.Context[k] = v
		}
	}
	ot.logger.Warn(ot.event)
}

// Global logger instance
var defaultLogger *Logger

// Init initializes the global logger
func Init(service string) {
	defaultLogger = NewLogger(service)
}

// Get returns the global logger instance
func Get() *Logger {
	if defaultLogger == nil {
		defaultLogger = NewLogger("excentrico-tools-go")
	}
	return defaultLogger
}

// Close closes the global logger's log file
func Close() error {
	if defaultLogger != nil {
		return defaultLogger.Close()
	}
	return nil
}
