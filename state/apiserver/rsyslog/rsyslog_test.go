// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package rsyslog_test

import (
	"encoding/pem"

	jc "github.com/juju/testing/checkers"
	gc "launchpad.net/gocheck"

	"github.com/juju/core/instance"
	"github.com/juju/core/juju/testing"
	"github.com/juju/core/state"
	"github.com/juju/core/state/api/params"
	apirsyslog "github.com/juju/core/state/api/rsyslog"
	"github.com/juju/core/state/apiserver/common"
	commontesting "github.com/juju/core/state/apiserver/common/testing"
	"github.com/juju/core/state/apiserver/rsyslog"
	apiservertesting "github.com/juju/core/state/apiserver/testing"
	coretesting "github.com/juju/core/testing"
)

type rsyslogSuite struct {
	testing.JujuConnSuite
	*commontesting.EnvironWatcherTest
	authorizer apiservertesting.FakeAuthorizer
	resources  *common.Resources
	rsyslog    *rsyslog.RsyslogAPI
}

var _ = gc.Suite(&rsyslogSuite{})

func (s *rsyslogSuite) SetUpTest(c *gc.C) {
	s.JujuConnSuite.SetUpTest(c)
	s.authorizer = apiservertesting.FakeAuthorizer{
		LoggedIn:       true,
		EnvironManager: true,
		MachineAgent:   true,
	}
	s.resources = common.NewResources()
	api, err := rsyslog.NewRsyslogAPI(s.State, s.resources, s.authorizer)
	c.Assert(err, gc.IsNil)
	s.EnvironWatcherTest = commontesting.NewEnvironWatcherTest(
		api, s.State, s.resources, commontesting.NoSecrets)
}

func verifyRsyslogCACert(c *gc.C, st *apirsyslog.State, expected string) {
	cfg, err := st.GetRsyslogConfig("foo")
	c.Assert(err, gc.IsNil)
	c.Assert(cfg.CACert, gc.DeepEquals, expected)
}

func (s *rsyslogSuite) TestSetRsyslogCert(c *gc.C) {
	st, m := s.OpenAPIAsNewMachine(c, state.JobManageEnviron)
	err := m.SetAddresses(instance.NewAddress("0.1.2.3", instance.NetworkUnknown))
	c.Assert(err, gc.IsNil)

	err = st.Rsyslog().SetRsyslogCert(coretesting.CACert)
	c.Assert(err, gc.IsNil)
	verifyRsyslogCACert(c, st.Rsyslog(), coretesting.CACert)
}

func (s *rsyslogSuite) TestSetRsyslogCertNil(c *gc.C) {
	st, m := s.OpenAPIAsNewMachine(c, state.JobManageEnviron)
	err := m.SetAddresses(instance.NewAddress("0.1.2.3", instance.NetworkUnknown))
	c.Assert(err, gc.IsNil)

	err = st.Rsyslog().SetRsyslogCert("")
	c.Assert(err, gc.ErrorMatches, "no certificates found")
	verifyRsyslogCACert(c, st.Rsyslog(), "")
}

func (s *rsyslogSuite) TestSetRsyslogCertInvalid(c *gc.C) {
	st, m := s.OpenAPIAsNewMachine(c, state.JobManageEnviron)
	err := m.SetAddresses(instance.NewAddress("0.1.2.3", instance.NetworkUnknown))
	c.Assert(err, gc.IsNil)

	err = st.Rsyslog().SetRsyslogCert(string(pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: []byte("not a valid certificate"),
	})))
	c.Assert(err, gc.ErrorMatches, ".*structure error.*")
	verifyRsyslogCACert(c, st.Rsyslog(), "")
}

func (s *rsyslogSuite) TestSetRsyslogCertPerms(c *gc.C) {
	// create a machine-0 so we have an addresss to log to
	m, err := s.State.AddMachine("trusty", state.JobManageEnviron)
	c.Assert(err, gc.IsNil)
	err = m.SetAddresses(instance.NewAddress("0.1.2.3", instance.NetworkUnknown))
	c.Assert(err, gc.IsNil)

	unitState, _ := s.OpenAPIAsNewMachine(c, state.JobHostUnits)
	err = unitState.Rsyslog().SetRsyslogCert(coretesting.CACert)
	c.Assert(err, gc.ErrorMatches, "invalid entity name or password")
	c.Assert(err, jc.Satisfies, params.IsCodeUnauthorized)
	// Verify no change was effected.
	verifyRsyslogCACert(c, unitState.Rsyslog(), "")
}

func (s *rsyslogSuite) TestUpgraderAPIAllowsUnitAgent(c *gc.C) {
	anAuthorizer := s.authorizer
	anAuthorizer.UnitAgent = true
	anAuthorizer.MachineAgent = false
	anUpgrader, err := rsyslog.NewRsyslogAPI(s.State, s.resources, anAuthorizer)
	c.Check(err, gc.IsNil)
	c.Check(anUpgrader, gc.NotNil)
}

func (s *rsyslogSuite) TestUpgraderAPIRefusesNonUnitNonMachineAgent(c *gc.C) {
	anAuthorizer := s.authorizer
	anAuthorizer.UnitAgent = false
	anAuthorizer.MachineAgent = false
	anUpgrader, err := rsyslog.NewRsyslogAPI(s.State, s.resources, anAuthorizer)
	c.Check(err, gc.NotNil)
	c.Check(anUpgrader, gc.IsNil)
	c.Assert(err, gc.ErrorMatches, "permission denied")
}
