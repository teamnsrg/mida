package log

import (
	"errors"
	"github.com/sirupsen/logrus"
	"github.com/snowzach/rotatefilehook"
	"os"
)

// The global log used throughout MIDA (not to be confused with individual logs for each crawl)
var Log = logrus.New()

func InitGlobalLogger(logfile string) {
	fileFormatter := new(logrus.TextFormatter)
	fileFormatter.FullTimestamp = true
	fileFormatter.DisableColors = true

	rotateFileHook, err := rotatefilehook.NewRotateFileHook(rotatefilehook.RotateFileConfig{
		Filename:   logfile,
		MaxSize:    50, //megabytes
		MaxBackups: 3,
		MaxAge:     180, //days
		Level:      logrus.DebugLevel,
		Formatter:  fileFormatter,
	})
	if err != nil {
		Log.Fatal("Logging initialization error: ", err)
	}

	consoleFormatter := new(logrus.TextFormatter)
	consoleFormatter.FullTimestamp = false
	consoleFormatter.ForceColors = true

	Log.SetOutput(os.Stdout)
	Log.SetFormatter(consoleFormatter)
	Log.AddHook(rotateFileHook)
}

// Helper function to setup logging using parameters from Cobra command
func ConfigureLogging(level int) error {
	switch level {
	case 0:
		Log.SetLevel(logrus.ErrorLevel)
	case 1:
		Log.SetLevel(logrus.WarnLevel)
	case 2:
		Log.SetLevel(logrus.InfoLevel)
	case 3:
		Log.SetLevel(logrus.DebugLevel)
		Log.SetReportCaller(true)
	default:
		return errors.New("invalid log level (Valid values: 0, 1, 2, 3)")
	}
	return nil
}
