// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package environment_test

import (
	gc "launchpad.net/gocheck"

	"github.com/juju/core/juju/testing"
	"github.com/juju/core/state"
	"github.com/juju/core/state/apiserver/common"
	commontesting "github.com/juju/core/state/apiserver/common/testing"
	"github.com/juju/core/state/apiserver/environment"
	apiservertesting "github.com/juju/core/state/apiserver/testing"
)

type environmentSuite struct {
	testing.JujuConnSuite
	*commontesting.EnvironWatcherTest

	authorizer apiservertesting.FakeAuthorizer
	resources  *common.Resources

	machine0 *state.Machine
	api      *environment.EnvironmentAPI
}

var _ = gc.Suite(&environmentSuite{})

func (s *environmentSuite) SetUpTest(c *gc.C) {
	s.JujuConnSuite.SetUpTest(c)

	var err error
	s.machine0, err = s.State.AddMachine("quantal", state.JobHostUnits, state.JobManageEnviron)
	c.Assert(err, gc.IsNil)

	s.authorizer = apiservertesting.FakeAuthorizer{
		Tag:          s.machine0.Tag(),
		LoggedIn:     true,
		MachineAgent: true,
		Entity:       s.machine0,
	}
	s.resources = common.NewResources()

	s.api, err = environment.NewEnvironmentAPI(
		s.State,
		s.resources,
		s.authorizer,
	)
	c.Assert(err, gc.IsNil)
	s.EnvironWatcherTest = commontesting.NewEnvironWatcherTest(
		s.api, s.State, s.resources, commontesting.NoSecrets)
}
