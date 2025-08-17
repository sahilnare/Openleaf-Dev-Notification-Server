package helpers

import (
	"encoding/json"
	"os"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	Log             = logrus.New()
	CombinedLogger  *logrus.Entry
	ExceptionLogger *logrus.Entry
)

func InitLogger() {

	if _, err := os.Stat("./logs"); os.IsNotExist(err) {
		err := os.Mkdir("./logs", 0755)
		if err != nil {
			panic("Failed to create logs directory: " + err.Error())
		}
	}

	Log.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "02-01-2006 15:04:05",
		DataKey:         "log_data",
		PrettyPrint:     true,
	})
	Log.SetReportCaller(false)
	Log.SetOutput(os.Stdout)

	Log.SetLevel(logrus.DebugLevel)

	// Setup files
	combinedLog := &lumberjack.Logger{
		Filename:   "./logs/combined.log",
		MaxSize:    150, // 150 MB
		MaxBackups: 5,
		MaxAge:     30,
		Compress:   true,
	}
	exceptionLog := &lumberjack.Logger{
		Filename:   "./logs/exceptions.log",
		MaxSize:    50, // 100 MB
		MaxBackups: 5,
		MaxAge:     15,
		Compress:   true,
	}

	// Set default combined log output
	Log.SetOutput(combinedLog)
	CombinedLogger = logrus.NewEntry(Log)

	// Exception logger
	exc := logrus.New()
	exc.SetFormatter(Log.Formatter)
	exc.SetOutput(exceptionLog)
	exc.SetLevel(logrus.ErrorLevel)
	exc.SetReportCaller(false)
	ExceptionLogger = logrus.NewEntry(exc)
}

// Helper function to convert int to string without fmt
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[pos:])
}

// Adds the filename as a top-level field (not inside log_data)
func addTopLevelFilenameField(fields logrus.Fields, skip int) logrus.Fields {
	_, file, line, ok := runtime.Caller(skip)
	filename := ""
	if ok {
		filename = file + ":" + itoa(line)
	}
	// Copy fields to avoid mutating input
	newFields := logrus.Fields{}
	for k, v := range fields {
		newFields[k] = v
	}
	newFields["__filename"] = filename // Use a unique key to avoid collision with log_data
	return newFields
}

// isJSONString checks if a string looks like JSON and returns the parsed object if it is
func isJSONString(value interface{}) (interface{}, bool) {
	if str, ok := value.(string); ok {
		str = strings.TrimSpace(str)
		if (strings.HasPrefix(str, "{") && strings.HasSuffix(str, "}")) ||
			(strings.HasPrefix(str, "[") && strings.HasSuffix(str, "]")) {
			var parsed interface{}
			if err := json.Unmarshal([]byte(str), &parsed); err == nil {
				return parsed, true
			}
		}
	}
	return value, false
}

// processValue recursively processes values to parse JSON strings
func processValue(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, val := range v {
			result[key] = processValue(val)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, val := range v {
			result[i] = processValue(val)
		}
		return result
	default:
		if parsed, isJSON := isJSONString(value); isJSON {
			return parsed
		}
		return value
	}
}

// toFields: If a single map[string]interface{} or logrus.Fields or map[string]string is passed, use as log_data object.
// If multiple keyValues, treat as key-value pairs for log_data object.
// Always returns a map with a single key "log_data" whose value is the object to be logged.
func toFields(keyValues ...interface{}) logrus.Fields {
	logData := map[string]interface{}{}

	if len(keyValues) == 1 {
		switch v := keyValues[0].(type) {
		case logrus.Fields:
			for k, val := range v {
				logData[k] = processValue(val)
			}
		case map[string]interface{}:
			for k, val := range v {
				logData[k] = processValue(val)
			}
		case map[string]string:
			for k, val := range v {
				logData[k] = processValue(val)
			}
		default:
			// If it's a struct or other type, just put as "value"
			logData["value"] = processValue(v)
		}
	} else if len(keyValues) > 1 {
		for i := 0; i < len(keyValues)-1; i += 2 {
			if key, ok := keyValues[i].(string); ok {
				logData[key] = processValue(keyValues[i+1])
			}
		}
	}

	return logrus.Fields{
		"log_data": logData,
	}
}

// Helper to move __filename to top-level "filename" and remove from log_data
func withTopLevelFilename(entry *logrus.Entry, skip int) *logrus.Entry {
	// Add __filename to fields
	fields := addTopLevelFilenameField(entry.Data, skip)
	// Remove __filename from log_data if present
	logData, hasLogData := fields["log_data"].(map[string]interface{})
	if hasLogData {
		if _, ok := logData["filename"]; ok {
			delete(logData, "filename")
		}
	}
	// Move __filename to top-level "filename"
	if filename, ok := fields["__filename"]; ok {
		fields["filename"] = filename
		delete(fields, "__filename")
	}
	return entry.WithFields(fields)
}

// === Combined log ===
func LogInfo(msg string, keyValues ...interface{}) {
	fields := toFields(keyValues...)
	entry := CombinedLogger.WithFields(fields)
	withTopLevelFilename(entry, 3).Info(msg)
}
func LogDebug(msg string, keyValues ...interface{}) {
	fields := toFields(keyValues...)
	entry := CombinedLogger.WithFields(fields)
	withTopLevelFilename(entry, 3).Debug(msg)
}
func LogWarn(msg string, keyValues ...interface{}) {
	fields := toFields(keyValues...)
	entry := CombinedLogger.WithFields(fields)
	withTopLevelFilename(entry, 3).Warn(msg)
}
func LogFatal(msg string, keyValues ...interface{}) {
	fields := toFields(keyValues...)
	entry := CombinedLogger.WithFields(fields)
	withTopLevelFilename(entry, 3).Fatal(msg)
}

// === Exception log ===
func LogException(msg string, keyValues ...interface{}) {
	fields := toFields(keyValues...)
	entry := ExceptionLogger.WithFields(fields)
	withTopLevelFilename(entry, 3).Error(msg)
}

// === Context helpers ===
// WithField returns a new log entry with a single field added to the CombinedLogger
func WithField(key string, value interface{}) *logrus.Entry {
	return CombinedLogger.WithField(key, value)
}

// WithFields returns a new log entry with multiple fields added to the CombinedLogger
func WithFields(fields logrus.Fields) *logrus.Entry {
	return CombinedLogger.WithFields(fields)
}

// WithExceptionField returns a new log entry with a single field added to the ExceptionLogger
func WithExceptionField(key string, value interface{}) *logrus.Entry {
	return ExceptionLogger.WithField(key, value)
}

// WithExceptionFields returns a new log entry with multiple fields added to the ExceptionLogger
func WithExceptionFields(fields logrus.Fields) *logrus.Entry {
	return ExceptionLogger.WithFields(fields)
}
