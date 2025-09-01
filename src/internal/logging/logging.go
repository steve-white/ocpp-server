package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	Logger           *log.Logger        = nil
	lumberjackLogger *lumberjack.Logger = nil
)

type myFormatter struct {
	log.TextFormatter
}

func ToJson(obj any) string {
	ret, _ := json.MarshalIndent(obj, " ", " ")
	return string(ret)
}

func LoggingSetup(isDebug bool, fileName string) *log.Logger {
	if Logger != nil {
		Logger.Writer().Close()
	}
	if lumberjackLogger != nil {
		lumberjackLogger.Close()
	}

	logLevel := log.InfoLevel

	if isDebug {
		logLevel = log.DebugLevel
	}

	lumberjackLogger = &lumberjack.Logger{
		Filename:   filepath.ToSlash("./logs/" + fileName + ".log"),
		MaxSize:    10, // MB
		MaxBackups: 10,
		//MaxAge:     30, // days
		Compress: false,
	}

	/*Logger = &log.Logger{
		Out:   lumberjackLogger,
		Level: logLevel,
		Hooks: make(log.LevelHooks),
		Formatter: &myFormatter{log.TextFormatter{
			FullTimestamp:          true,
			TimestampFormat:        "2006-01-02 15:04:05.000",
			ForceColors:            true,
			DisableLevelTruncation: true,
		}},
	}*/
	Logger = &log.Logger{
		Out:   io.MultiWriter(os.Stderr, lumberjackLogger),
		Level: logLevel,
		Hooks: make(log.LevelHooks),
		Formatter: &myFormatter{log.TextFormatter{
			FullTimestamp:          true,
			TimestampFormat:        "2006-01-02 15:04:05.000",
			ForceColors:            true,
			DisableLevelTruncation: true,
		}},
	}
	return Logger
}

func (f *myFormatter) Format(entry *log.Entry) ([]byte, error) {
	var logLevelStr string

	switch entry.Level {
	case log.InfoLevel:
		logLevelStr = "INFO "
	case log.WarnLevel:
		logLevelStr = "WARN "
	case log.DebugLevel:
		logLevelStr = "DEBUG"
	case log.TraceLevel:
		logLevelStr = "TRACE"
	case log.FatalLevel:
		logLevelStr = "FATAL"
	case log.PanicLevel:
		logLevelStr = "PANIC"
	default:
		logLevelStr = strings.ToUpper(entry.Level.String())
	}

	return []byte(fmt.Sprintf("%s : %s : %s\n", entry.Time.Format(f.TimestampFormat), logLevelStr, entry.Message)), nil
}
