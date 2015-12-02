package logger

import (
	"fmt"
	"io"
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
)

var (
	levelMap = map[string]logutils.LogLevel{
		"debug": DEBUG,
		"info":  INFO,
		"warn":  WARN,
		"error": ERROR,
		"fatal": FATAL,
	}

	levels = []logutils.LogLevel{DEBUG, INFO, WARN, ERROR, FATAL}
	Levels = []string{"fatal", "error", "warn", "info", "debug"}

	filter *logutils.LevelFilter
)

func Configure(lvl, prefix string, writer io.Writer) {
	if writer == nil {
		writer = os.Stdout
	}
	filter = &logutils.LevelFilter{
		Levels:   levels,
		MinLevel: parse(lvl),
		Writer:   writer,
	}
	log.SetOutput(filter)
	log.SetPrefix(prefix)
}

func Level() logutils.LogLevel {
	return filter.MinLevel
}

func SetLevel(lvl string) {
	filter.SetMinLevel(parse(lvl))
}

func Errorf(f string, m ...interface{}) { writeLog("error", f, m...) }
func Warnf(f string, m ...interface{})  { writeLog("warn", f, m...) }
func Infof(f string, m ...interface{})  { writeLog("info", f, m...) }
func Debugf(f string, m ...interface{}) {
	if Level() == DEBUG {
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
	if lvl, ok := levelMap[l]; ok {
		return lvl
	}
	return INFO
}
