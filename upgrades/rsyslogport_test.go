// Copyright 2014 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package upgrades_test

import (
	gc "launchpad.net/gocheck"

	"github.com/juju/core/environs/config"
	jujutesting "github.com/juju/core/juju/testing"
	"github.com/juju/core/state"
	"github.com/juju/core/upgrades"
)

type rsyslogPortSuite struct {
	jujutesting.JujuConnSuite
	ctx upgrades.Context
}

var _ = gc.Suite(&rsyslogPortSuite{})

func (s *rsyslogPortSuite) SetUpTest(c *gc.C) {
	s.JujuConnSuite.SetUpTest(c)
	apiState, _ := s.OpenAPIAsNewMachine(c, state.JobManageEnviron)
	s.ctx = &mockContext{
		agentConfig: &mockAgentConfig{
			dataDir:   s.DataDir(),
			stateInfo: s.StateInfo(c),
		},
		apiState: apiState,
		state:    s.State,
	}
	cfg, err := s.State.EnvironConfig()
	c.Assert(err, gc.IsNil)
	c.Assert(cfg.SyslogPort(), gc.Not(gc.Equals), config.DefaultSyslogPort)
}

func (s *rsyslogPortSuite) TestSyslogPortChanged(c *gc.C) {
	err := upgrades.UpdateRsyslogPort(s.ctx)
	c.Assert(err, gc.IsNil)
	cfg, err := s.State.EnvironConfig()
	c.Assert(err, gc.IsNil)
	c.Assert(cfg.SyslogPort(), gc.Equals, config.DefaultSyslogPort)
}

func (s *rsyslogPortSuite) TestIdempotent(c *gc.C) {
	err := upgrades.UpdateRsyslogPort(s.ctx)
	c.Assert(err, gc.IsNil)
	err = upgrades.UpdateRsyslogPort(s.ctx)
	c.Assert(err, gc.IsNil)
	cfg, err := s.State.EnvironConfig()
	c.Assert(err, gc.IsNil)
	c.Assert(cfg.SyslogPort(), gc.Equals, config.DefaultSyslogPort)
}
