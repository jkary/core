// Copyright 2014 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package machineenvironmentworker_test

import (
	"io/ioutil"
	"os"
	"path"
	"time"

	jc "github.com/juju/testing/checkers"
	gc "launchpad.net/gocheck"

	"github.com/juju/core/agent"
	"github.com/juju/core/environs/config"
	jujutesting "github.com/juju/core/juju/testing"
	"github.com/juju/core/names"
	"github.com/juju/core/provider"
	"github.com/juju/core/state"
	"github.com/juju/core/state/api"
	"github.com/juju/core/state/api/environment"
	"github.com/juju/core/testing"
	"github.com/juju/core/utils/apt"
	"github.com/juju/core/utils/proxy"
	"github.com/juju/core/worker"
	"github.com/juju/core/worker/machineenvironmentworker"
)

type MachineEnvironmentWatcherSuite struct {
	jujutesting.JujuConnSuite

	apiRoot        *api.State
	environmentAPI *environment.Facade
	machine        *state.Machine

	proxyFile string
	started   bool
}

var _ = gc.Suite(&MachineEnvironmentWatcherSuite{})

func (s *MachineEnvironmentWatcherSuite) setStarted() {
	s.started = true
}

func (s *MachineEnvironmentWatcherSuite) SetUpTest(c *gc.C) {
	s.JujuConnSuite.SetUpTest(c)
	s.apiRoot, s.machine = s.OpenAPIAsNewMachine(c)
	// Create the machiner API facade.
	s.environmentAPI = s.apiRoot.Environment()
	c.Assert(s.environmentAPI, gc.NotNil)

	proxyDir := c.MkDir()
	s.PatchValue(&machineenvironmentworker.ProxyDirectory, proxyDir)
	s.started = false
	s.PatchValue(&machineenvironmentworker.Started, s.setStarted)
	s.PatchValue(&apt.ConfFile, path.Join(proxyDir, "juju-apt-proxy"))
	s.proxyFile = path.Join(proxyDir, machineenvironmentworker.ProxyFile)
}

func (s *MachineEnvironmentWatcherSuite) waitForPostSetup(c *gc.C) {
	for {
		select {
		case <-time.After(testing.LongWait):
			c.Fatalf("timeout while waiting for setup")
		case <-time.After(10 * time.Millisecond):
			if s.started {
				return
			}
		}
	}
}

func (s *MachineEnvironmentWatcherSuite) waitProxySettings(c *gc.C, expected proxy.Settings) {
	for {
		select {
		case <-time.After(testing.LongWait):
			c.Fatalf("timeout while waiting for proxy settings to change")
		case <-time.After(10 * time.Millisecond):
			obtained := proxy.DetectProxies()
			if obtained != expected {
				c.Logf("proxy settings are %#v, still waiting", obtained)
				continue
			}
			return
		}
	}
}

func (s *MachineEnvironmentWatcherSuite) waitForFile(c *gc.C, filename, expected string) {
	for {
		select {
		case <-time.After(testing.LongWait):
			c.Fatalf("timeout while waiting for proxy settings to change")
		case <-time.After(10 * time.Millisecond):
			fileContent, err := ioutil.ReadFile(filename)
			if os.IsNotExist(err) {
				continue
			}
			c.Assert(err, gc.IsNil)
			if string(fileContent) != expected {
				c.Logf("file content not matching, still waiting")
				continue
			}
			return
		}
	}
}

func (s *MachineEnvironmentWatcherSuite) makeWorker(c *gc.C, agentConfig agent.Config) worker.Worker {
	return machineenvironmentworker.NewMachineEnvironmentWorker(s.environmentAPI, agentConfig)
}

func (s *MachineEnvironmentWatcherSuite) TestRunStop(c *gc.C) {
	agentConfig := agentConfig("0", "ec2")
	envWorker := s.makeWorker(c, agentConfig)
	c.Assert(worker.Stop(envWorker), gc.IsNil)
}

func (s *MachineEnvironmentWatcherSuite) updateConfig(c *gc.C) (proxy.Settings, proxy.Settings) {

	proxySettings := proxy.Settings{
		Http:    "http proxy",
		Https:   "https proxy",
		Ftp:     "ftp proxy",
		NoProxy: "no proxy",
	}
	attrs := map[string]interface{}{}
	for k, v := range config.ProxyConfigMap(proxySettings) {
		attrs[k] = v
	}

	// We explicitly set apt proxy settings as well to show that it is the apt
	// settings that are used for the apt config, and not just the normal
	// proxy settings which is what we would get if we don't explicitly set
	// apt values.
	aptProxySettings := proxy.Settings{
		Http:  "apt http proxy",
		Https: "apt https proxy",
		Ftp:   "apt ftp proxy",
	}
	for k, v := range config.AptProxyConfigMap(aptProxySettings) {
		attrs[k] = v
	}

	err := s.State.UpdateEnvironConfig(attrs, nil, nil)
	c.Assert(err, gc.IsNil)

	return proxySettings, aptProxySettings
}

func (s *MachineEnvironmentWatcherSuite) TestInitialState(c *gc.C) {
	proxySettings, aptProxySettings := s.updateConfig(c)

	agentConfig := agentConfig("0", "ec2")
	envWorker := s.makeWorker(c, agentConfig)
	defer worker.Stop(envWorker)

	s.waitProxySettings(c, proxySettings)
	s.waitForFile(c, s.proxyFile, proxySettings.AsScriptEnvironment()+"\n")
	s.waitForFile(c, apt.ConfFile, apt.ProxyContent(aptProxySettings)+"\n")
}

func (s *MachineEnvironmentWatcherSuite) TestRespondsToEvents(c *gc.C) {
	agentConfig := agentConfig("0", "ec2")
	envWorker := s.makeWorker(c, agentConfig)
	defer worker.Stop(envWorker)
	s.waitForPostSetup(c)

	proxySettings, aptProxySettings := s.updateConfig(c)

	s.waitProxySettings(c, proxySettings)
	s.waitForFile(c, s.proxyFile, proxySettings.AsScriptEnvironment()+"\n")
	s.waitForFile(c, apt.ConfFile, apt.ProxyContent(aptProxySettings)+"\n")
}

func (s *MachineEnvironmentWatcherSuite) TestInitialStateLocalMachine1(c *gc.C) {
	proxySettings, aptProxySettings := s.updateConfig(c)

	agentConfig := agentConfig("1", provider.Local)
	envWorker := s.makeWorker(c, agentConfig)
	defer worker.Stop(envWorker)

	s.waitProxySettings(c, proxySettings)
	s.waitForFile(c, s.proxyFile, proxySettings.AsScriptEnvironment()+"\n")
	s.waitForFile(c, apt.ConfFile, apt.ProxyContent(aptProxySettings)+"\n")
}

func (s *MachineEnvironmentWatcherSuite) TestInitialStateLocalMachine0(c *gc.C) {
	proxySettings, _ := s.updateConfig(c)

	agentConfig := agentConfig("0", provider.Local)
	envWorker := s.makeWorker(c, agentConfig)
	defer worker.Stop(envWorker)
	s.waitForPostSetup(c)

	s.waitProxySettings(c, proxySettings)

	c.Assert(apt.ConfFile, jc.DoesNotExist)
	c.Assert(s.proxyFile, jc.DoesNotExist)
}

type mockConfig struct {
	agent.Config
	tag      string
	provider string
}

func (mock *mockConfig) Tag() string {
	return mock.tag
}

func (mock *mockConfig) Value(key string) string {
	if key == agent.ProviderType {
		return mock.provider
	}
	return ""
}

func agentConfig(machineId, provider string) *mockConfig {
	return &mockConfig{tag: names.MachineTag(machineId), provider: provider}
}
