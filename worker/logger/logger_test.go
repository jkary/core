// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package logger_test

import (
	"time"

	"github.com/juju/loggo"
	gc "launchpad.net/gocheck"

	"github.com/juju/core/agent"
	"github.com/juju/core/juju/testing"
	"github.com/juju/core/state"
	"github.com/juju/core/state/api"
	apilogger "github.com/juju/core/state/api/logger"
	"github.com/juju/core/worker"
	"github.com/juju/core/worker/logger"
)

// worstCase is used for timeouts when timing out
// will fail the test. Raising this value should
// not affect the overall running time of the tests
// unless they fail.
const worstCase = 5 * time.Second

type LoggerSuite struct {
	testing.JujuConnSuite

	apiRoot   *api.State
	loggerApi *apilogger.State
	machine   *state.Machine
}

var _ = gc.Suite(&LoggerSuite{})

func (s *LoggerSuite) SetUpTest(c *gc.C) {
	s.JujuConnSuite.SetUpTest(c)
	s.apiRoot, s.machine = s.OpenAPIAsNewMachine(c)
	// Create the machiner API facade.
	s.loggerApi = s.apiRoot.Logger()
	c.Assert(s.loggerApi, gc.NotNil)
}

func (s *LoggerSuite) waitLoggingInfo(c *gc.C, expected string) {
	timeout := time.After(worstCase)
	for {
		select {
		case <-timeout:
			c.Fatalf("timeout while waiting for logging info to change")
		case <-time.After(10 * time.Millisecond):
			loggerInfo := loggo.LoggerInfo()
			if loggerInfo != expected {
				c.Logf("logging is %q, still waiting", loggerInfo)
				continue
			}
			return
		}
	}
}

type mockConfig struct {
	agent.Config
	c   *gc.C
	tag string
}

func (mock *mockConfig) Tag() string {
	return mock.tag
}

func agentConfig(c *gc.C, tag string) *mockConfig {
	return &mockConfig{c: c, tag: tag}
}

func (s *LoggerSuite) makeLogger(c *gc.C) (worker.Worker, *mockConfig) {
	config := agentConfig(c, s.machine.Tag())
	return logger.NewLogger(s.loggerApi, config), config
}

func (s *LoggerSuite) TestRunStop(c *gc.C) {
	loggingWorker, _ := s.makeLogger(c)
	c.Assert(worker.Stop(loggingWorker), gc.IsNil)
}

func (s *LoggerSuite) TestInitialState(c *gc.C) {
	config, err := s.State.EnvironConfig()
	c.Assert(err, gc.IsNil)
	expected := config.LoggingConfig()

	initial := "<root>=DEBUG;wibble=ERROR"
	c.Assert(expected, gc.Not(gc.Equals), initial)

	loggo.ResetLoggers()
	err = loggo.ConfigureLoggers(initial)
	c.Assert(err, gc.IsNil)

	loggingWorker, _ := s.makeLogger(c)
	defer worker.Stop(loggingWorker)

	s.waitLoggingInfo(c, expected)
}
