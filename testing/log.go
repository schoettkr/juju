package testing

import (
	"launchpad.net/gocheck"
	"launchpad.net/juju-core/log"
)

// LoggingSuite redirects the juju logger to the test logger
// when embedded in a gocheck suite type.
type LoggingSuite struct {
	oldTarget log.Logger
	oldDebug  bool
}

func (t *LoggingSuite) SetUpTest(c *gocheck.C) {
	t.oldTarget = log.Target
	t.oldDebug = log.Debug
	log.Debug = true
	log.Target = c
}

func (t *LoggingSuite) TearDownTest(c *gocheck.C) {
	log.Target = t.oldTarget
	log.Debug = t.oldDebug
}
