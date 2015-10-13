package main

import (
	"fmt"
	"log"
	"os"

	"github.com/hashicorp/logutils"
)

var logPrefix = "[romulusd] "
var logLevels = []string{"fatal", "error", "warn", "info", "debug"}

func setupLog() {
	lvls := make([]logutils.LogLevel, len(logLevels))
	for i := range logLevels {
		lvls = append(lvls, logutils.LogLevel(logLevels[i]))
	}
	f := &logutils.LevelFilter{
		Levels:   lvls,
		MinLevel: logutils.LogLevel(*logLevel),
		Writer:   os.Stdout,
	}
	log.SetOutput(f)
	log.SetPrefix(logPrefix)
}

func writeLog(p, f string, m ...interface{}) {
	var msg = f
	if m != nil && len(m) > 0 {
		msg = fmt.Sprintf(f, m...)
	}
	log.Printf("[%s] %s", p, msg)
}

func errorL(f string, m ...interface{}) { writeLog("error", f, m...) }
func warnL(f string, m ...interface{})  { writeLog("warn", f, m...) }
func infoL(f string, m ...interface{})  { writeLog("info", f, m...) }
func debugL(f string, m ...interface{}) {
	if *debug || *logLevel == "debug" {
		writeLog("debug", f, m...)
	}
}
func fatalL(f string, m ...interface{}) {
	writeLog("fatal", f, m...)
	os.Exit(1)
}
