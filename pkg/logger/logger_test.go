package logger

import (
"bytes"
"log"
"os"
"testing"
)

func TestLogger(t *testing.T) {
var buf bytes.Buffer
log.SetOutput(&buf)
defer log.SetOutput(os.Stderr)

Debug = true
Debugf("debug %s", "message")
if !bytes.Contains(buf.Bytes(), []byte("[DEBUG] debug message")) {
t.Errorf("expected debug message, got %s", buf.String())
}

buf.Reset()
Debug = false
Debugf("debug %s", "message")
if buf.Len() > 0 {
t.Errorf("expected empty buffer, got %s", buf.String())
}

buf.Reset()
Printf("print %s", "message")
if !bytes.Contains(buf.Bytes(), []byte("print message")) {
t.Errorf("expected print message, got %s", buf.String())
}

buf.Reset()
Println("println message")
if !bytes.Contains(buf.Bytes(), []byte("println message")) {
t.Errorf("expected println message, got %s", buf.String())
}
}
func TestFatalHelpers(t *testing.T) {
oldLogFatal := logFatal
logFatal = func(v ...interface{}) {}
defer func() { logFatal = oldLogFatal }()
Fatal("fatal test")

oldLogFatalf := logFatalf
logFatalf = func(format string, v ...interface{}) {}
defer func() { logFatalf = oldLogFatalf }()
Fatalf("fatal %s", "test")
}
