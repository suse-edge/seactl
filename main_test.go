package main

import (
"testing"
"github.com/stretchr/testify/assert"
)

func TestNewCommand(t *testing.T) {
cmd := newCommand()
assert.NotNil(t, cmd)
assert.Equal(t, "seactl", cmd.Use)
}

func TestMainContext(t *testing.T) {
oldOsExit := osExit
osExit = func(code int) {}
defer func() { osExit = oldOsExit }()

cmd := newCommand()
cmd.Run(cmd, nil)
}
