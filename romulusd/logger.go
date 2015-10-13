package main

import (
	"fmt"
	"log"
	"os"

	"github.com/hashicorp/logutils"
)

var logPrefix = "[romulusd]"
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

func writeLog(f string, m ...interface{}) {
	log.Printf(f, m)
}

func errorL(f string, m ...interface{}) { writeLog("[error] %s", fmt.Sprintf(f, m)) }
func warnL(f string, m ...interface{})  { writeLog("[warn] %s", fmt.Sprintf(f, m)) }
func infoL(f string, m ...interface{})  { writeLog("[info] %s", fmt.Sprintf(f, m)) }
func debugL(f string, m ...interface{}) {
	if *debug || *logLevel == "debug" {
		writeLog("[debug] %s", fmt.Sprintf(f, m))
	}
}
func fatalL(f string, m ...interface{}) {
	writeLog("[fatal]", fmt.Sprintf(f, m))
	os.Exit(1)
}
