// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

// TODO(wallyworld) - move to instancepoller_test
package instancepoller

import (
	"strings"
	"time"

	"github.com/juju/loggo"
	gc "launchpad.net/gocheck"

	"launchpad.net/juju-core/juju/testing"
	"launchpad.net/juju-core/state"
	coretesting "launchpad.net/juju-core/testing"
)

var _ = gc.Suite(&observerSuite{})

type observerSuite struct {
	testing.JujuConnSuite
}

func (s *observerSuite) TestWaitsForValidEnviron(c *gc.C) {
	obs, err := newEnvironObserver(s.State, nil)
	c.Assert(err, gc.IsNil)
	env := obs.Environ()
	stateConfig, err := s.State.EnvironConfig()
	c.Assert(err, gc.IsNil)
	c.Assert(env.Config().AllAttrs(), gc.DeepEquals, stateConfig.AllAttrs())
}

func (s *observerSuite) TestEnvironmentChanges(c *gc.C) {
	originalConfig, err := s.State.EnvironConfig()
	c.Assert(err, gc.IsNil)

	logc := make(logChan, 1009)
	c.Assert(loggo.RegisterWriter("testing", logc, loggo.WARNING), gc.IsNil)
	defer loggo.RemoveWriter("testing")

	obs, err := newEnvironObserver(s.State, nil)
	c.Assert(err, gc.IsNil)

	env := obs.Environ()
	c.Assert(env.Config().AllAttrs(), gc.DeepEquals, originalConfig.AllAttrs())
	var oldType string
	oldType = env.Config().AllAttrs()["type"].(string)

	info := s.StateInfo(c)
	opts := state.DefaultDialOpts()
	st2, err := state.Open(info, opts, state.Policy(nil))
	defer st2.Close()

	// Change to an invalid configuration and check
	// that the observer's environment remains the same.
	st2.UpdateEnvironConfig(map[string]interface{}{"type": "invalid"}, nil, nil)
	st2.StartSync()

	// Wait for the observer to register the invalid environment
	timeout := time.After(coretesting.LongWait)
loop:
	for {
		select {
		case msg := <-logc:
			if strings.Contains(msg, "error creating Environ") {
				break loop
			}
		case <-timeout:
			c.Fatalf("timed out waiting to see broken environment")
		}
	}
	// Check that the returned environ is still the same.
	env = obs.Environ()
	c.Assert(env.Config().AllAttrs(), gc.DeepEquals, originalConfig.AllAttrs())

	// Change the environment back to a valid configuration
	// with a different name and check that we see it.
	st2.UpdateEnvironConfig(map[string]interface{}{"type": oldType, "name": "a-new-name"}, nil, nil)
	st2.StartSync()

	for a := coretesting.LongAttempt.Start(); a.Next(); {
		env := obs.Environ()
		if !a.HasNext() {
			c.Fatalf("timed out waiting for new environ")
		}
		if env.Config().Name() == "a-new-name" {
			break
		}
	}
}

type logChan chan string

func (logc logChan) Write(level loggo.Level, name, filename string, line int, timestamp time.Time, message string) {
	logc <- message
}
