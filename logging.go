package main

import (
	"github.com/sirupsen/logrus"
	"github.com/snowzach/rotatefilehook"
	//"github.com/x-cray/logrus-prefixed-formatter"
	"os"
)

var Log = logrus.New()

func InitLogger() {
	logLevel := logrus.InfoLevel

	//fileFormatter := new(prefixed.TextFormatter)
	fileFormatter := new(logrus.TextFormatter)
	fileFormatter.FullTimestamp = true
	fileFormatter.DisableColors = true

	rotateFileHook, err := rotatefilehook.NewRotateFileHook(rotatefilehook.RotateFileConfig{
		Filename:   MIDALogFile,
		MaxSize:    50, //megabytes
		MaxBackups: 3,
		MaxAge:     30, //days
		Level:      logLevel,
		Formatter:  fileFormatter,
	})
	if err != nil {
		Log.Fatal(err)
	}

	//consoleFormatter := new(prefixed.TextFormatter)
	consoleFormatter := new(logrus.TextFormatter)
	consoleFormatter.FullTimestamp = false
	consoleFormatter.ForceColors = true
	//consoleFormatter.ForceFormatting = true

	Log.SetLevel(logLevel)
	Log.SetOutput(os.Stdout)
	Log.SetFormatter(consoleFormatter)
	Log.SetReportCaller(false)
	Log.AddHook(rotateFileHook)
}
