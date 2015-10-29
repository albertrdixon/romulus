package logger

import (
	"fmt"
	"log"
	"os"

	"github.com/hashicorp/logutils"
)

const (
	DEBUG logutils.LogLevel = "debug"
	INFO  logutils.LogLevel = "info"
	WARN  logutils.LogLevel = "warn"
	ERROR logutils.LogLevel = "error"
	FATAL logutils.LogLevel = "fatal"

	prefix = "[romulusd] "
)

var (
	level  = INFO
	levels = []logutils.LogLevel{FATAL, ERROR, WARN, INFO, DEBUG}
)

func Configure(lvl string) {
	level = parse(lvl)
	f := &logutils.LevelFilter{
		Levels:   levels,
		MinLevel: level,
		Writer:   os.Stdout,
	}
	log.SetOutput(f)
	log.SetPrefix(prefix)
}

func Errorf(f string, m ...interface{}) { writeLog("error", f, m...) }
func Warnf(f string, m ...interface{})  { writeLog("warn", f, m...) }
func Infof(f string, m ...interface{})  { writeLog("info", f, m...) }
func Debugf(f string, m ...interface{}) {
	if level == DEBUG {
		writeLog("debug", f, m...)
	}
}
func Fatalf(f string, m ...interface{}) {
	writeLog("fatal", f, m...)
	os.Exit(1)
}

func writeLog(p, f string, m ...interface{}) {
	var msg = f
	if m != nil && len(m) > 0 {
		msg = fmt.Sprintf(f, m...)
	}
	log.Printf("[%s] %s", p, msg)
}

func parse(l string) logutils.LogLevel {
	for i := range levels {
		if logutils.LogLevel(l) == levels[i] {
			return levels[i]
		}
	}
	return INFO
}
