// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package main

import (
	"encoding/base64"
	"fmt"

	"launchpad.net/gnuflag"
	"launchpad.net/goyaml"

	"launchpad.net/juju-core/agent"
	"launchpad.net/juju-core/cmd"
	"launchpad.net/juju-core/constraints"
	"launchpad.net/juju-core/environs"
	"launchpad.net/juju-core/environs/config"
	"launchpad.net/juju-core/instance"
	"launchpad.net/juju-core/state"
	"launchpad.net/juju-core/state/api/params"
)

type BootstrapCommand struct {
	cmd.CommandBase
	Conf        AgentConf
	EnvConfig   map[string]interface{}
	Constraints constraints.Value
	Hardware    instance.HardwareCharacteristics
	InstanceId  string
}

// Info returns a decription of the command.
func (c *BootstrapCommand) Info() *cmd.Info {
	return &cmd.Info{
		Name:    "bootstrap-state",
		Purpose: "initialize juju state",
	}
}

func (c *BootstrapCommand) SetFlags(f *gnuflag.FlagSet) {
	c.Conf.addFlags(f)
	yamlBase64Var(f, &c.EnvConfig, "env-config", "", "initial environment configuration (yaml, base64 encoded)")
	f.Var(constraints.ConstraintsValue{&c.Constraints}, "constraints", "initial environment constraints (space-separated strings)")
	f.Var(&c.Hardware, "hardware", "hardware characteristics (space-separated strings)")
	f.StringVar(&c.InstanceId, "instance-id", "", "unique instance-id for bootstrap machine")
}

// Init initializes the command for running.
func (c *BootstrapCommand) Init(args []string) error {
	if len(c.EnvConfig) == 0 {
		return requiredError("env-config")
	}
	if c.InstanceId == "" {
		return requiredError("instance-id")
	}
	return c.Conf.checkArgs(args)
}

// Run initializes state for an environment.
func (c *BootstrapCommand) Run(_ *cmd.Context) error {
	envCfg, err := config.New(config.NoDefaults, c.EnvConfig)
	if err != nil {
		return err
	}
	if err := c.Conf.read("machine-0"); err != nil {
		return err
	}
	// agent.Jobs is an optional field in the agent config, and was
	// introduced after 1.17.2. We default to allowing units on
	// machine-0 if missing.
	jobs := c.Conf.config.Jobs()
	if len(jobs) == 0 {
		jobs = []params.MachineJob{
			params.JobManageEnviron,
			params.JobHostUnits,
		}
	}
	st, _, err := c.Conf.config.InitializeState(envCfg, agent.BootstrapMachineConfig{
		Constraints:     c.Constraints,
		Jobs:            jobs,
		InstanceId:      instance.Id(c.InstanceId),
		Characteristics: c.Hardware,
	}, state.DefaultDialOpts(), environs.NewStatePolicy())
	if err != nil {
		return err
	}
	st.Close()
	return nil
}

// yamlBase64Value implements gnuflag.Value on a map[string]interface{}.
type yamlBase64Value map[string]interface{}

// Set decodes the base64 value into yaml then expands that into a map.
func (v *yamlBase64Value) Set(value string) error {
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return err
	}
	return goyaml.Unmarshal(decoded, v)
}

func (v *yamlBase64Value) String() string {
	return fmt.Sprintf("%v", *v)
}

// yamlBase64Var sets up a gnuflag flag analogous to the FlagSet.*Var methods.
func yamlBase64Var(fs *gnuflag.FlagSet, target *map[string]interface{}, name string, value string, usage string) {
	fs.Var((*yamlBase64Value)(target), name, usage)
}
