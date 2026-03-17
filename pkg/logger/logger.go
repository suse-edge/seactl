package logger

import (
"log"
)

var logFatal = log.Fatal
var logFatalf = log.Fatalf

var Debug bool

func Debugf(format string, v ...interface{}) {
if Debug {
log.Printf("[DEBUG] " + format, v...)
}
}

func Printf(format string, v ...interface{}) {
log.Printf(format, v...)
}

func Println(v ...interface{}) {
log.Println(v...)
}

func Fatal(v ...interface{}) {
logFatal(v...)
}

func Fatalf(format string, v ...interface{}) {
logFatalf(format, v...)
}
