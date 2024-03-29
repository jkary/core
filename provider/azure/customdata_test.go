// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package azure

import (
	"encoding/base64"

	gc "launchpad.net/gocheck"

	"github.com/juju/core/agent"
	"github.com/juju/core/environs"
	"github.com/juju/core/environs/cloudinit"
	"github.com/juju/core/names"
	"github.com/juju/core/state"
	"github.com/juju/core/state/api"
	"github.com/juju/core/state/api/params"
	"github.com/juju/core/testing"
	"github.com/juju/core/tools"
)

type customDataSuite struct {
	testing.BaseSuite
}

var _ = gc.Suite(&customDataSuite{})

// makeMachineConfig produces a valid cloudinit machine config.
func makeMachineConfig(c *gc.C) *cloudinit.MachineConfig {
	machineID := "0"
	return &cloudinit.MachineConfig{
		MachineId:          machineID,
		MachineNonce:       "gxshasqlnng",
		DataDir:            environs.DataDir,
		LogDir:             agent.DefaultLogDir,
		Jobs:               []params.MachineJob{params.JobManageEnviron, params.JobHostUnits},
		CloudInitOutputLog: environs.CloudInitOutputLog,
		Tools:              &tools.Tools{URL: "file://" + c.MkDir()},
		StateInfo: &state.Info{
			CACert:   testing.CACert,
			Addrs:    []string{"127.0.0.1:123"},
			Tag:      names.MachineTag(machineID),
			Password: "password",
		},
		APIInfo: &api.Info{
			CACert: testing.CACert,
			Addrs:  []string{"127.0.0.1:123"},
			Tag:    names.MachineTag(machineID),
		},
		MachineAgentServiceName: "jujud-machine-0",
	}
}

// makeBadMachineConfig produces a cloudinit machine config that cloudinit
// will reject as invalid.
func makeBadMachineConfig() *cloudinit.MachineConfig {
	// As it happens, a default-initialized config is invalid.
	return &cloudinit.MachineConfig{}
}

func (*customDataSuite) TestMakeCustomDataPropagatesError(c *gc.C) {
	_, err := makeCustomData(makeBadMachineConfig())
	c.Assert(err, gc.NotNil)
	c.Check(err, gc.ErrorMatches, "failure while generating custom data: invalid machine configuration: invalid machine id")
}

func (*customDataSuite) TestMakeCustomDataEncodesUserData(c *gc.C) {
	cfg := makeMachineConfig(c)

	encodedData, err := makeCustomData(cfg)
	c.Assert(err, gc.IsNil)

	data, err := base64.StdEncoding.DecodeString(encodedData)
	c.Assert(err, gc.IsNil)
	reference, err := environs.ComposeUserData(cfg, nil)
	c.Assert(err, gc.IsNil)
	c.Check(data, gc.DeepEquals, reference)
}
