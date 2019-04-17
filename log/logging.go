package log

import (
	"errors"
	"github.com/sirupsen/logrus"
	"github.com/snowzach/rotatefilehook"
	"os"
)

var Log = logrus.New()

func InitLogger() {
	//fileFormatter := new(prefixed.TextFormatter)
	fileFormatter := new(logrus.TextFormatter)
	fileFormatter.FullTimestamp = true
	fileFormatter.DisableColors = true

	rotateFileHook, err := rotatefilehook.NewRotateFileHook(rotatefilehook.RotateFileConfig{
		Filename:   "mida.log",
		MaxSize:    50, //megabytes
		MaxBackups: 3,
		MaxAge:     30, //days
		Level:      logrus.InfoLevel,
		Formatter:  fileFormatter,
	})
	if err != nil {
		Log.Fatal(err)
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
