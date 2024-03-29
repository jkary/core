// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provisioner

import (
	"fmt"

	"github.com/juju/loggo"

	"github.com/juju/core/agent"
	"github.com/juju/core/container"
	"github.com/juju/core/container/lxc"
	"github.com/juju/core/environs"
	"github.com/juju/core/environs/network"
	"github.com/juju/core/instance"
	"github.com/juju/core/state/api/params"
	"github.com/juju/core/tools"
)

var lxcLogger = loggo.GetLogger("juju.provisioner.lxc")

var _ environs.InstanceBroker = (*lxcBroker)(nil)
var _ tools.HasTools = (*lxcBroker)(nil)

type APICalls interface {
	ContainerConfig() (params.ContainerConfig, error)
}

func NewLxcBroker(api APICalls, tools *tools.Tools, agentConfig agent.Config, managerConfig container.ManagerConfig) (environs.InstanceBroker, error) {
	manager, err := lxc.NewContainerManager(managerConfig)
	if err != nil {
		return nil, err
	}
	return &lxcBroker{
		manager:     manager,
		api:         api,
		tools:       tools,
		agentConfig: agentConfig,
	}, nil
}

type lxcBroker struct {
	manager     container.Manager
	api         APICalls
	tools       *tools.Tools
	agentConfig agent.Config
}

func (broker *lxcBroker) Tools(series string) tools.List {
	// TODO: thumper 2014-04-08 bug 1304151
	// should use the api get get tools for the series.
	seriesTools := *broker.tools
	seriesTools.Version.Series = series
	return tools.List{&seriesTools}
}

// StartInstance is specified in the Broker interface.
func (broker *lxcBroker) StartInstance(args environs.StartInstanceParams) (instance.Instance, *instance.HardwareCharacteristics, []network.Info, error) {
	if args.MachineConfig.HasNetworks() {
		return nil, nil, nil, fmt.Errorf("starting lxc containers with networks is not supported yet.")
	}
	// TODO: refactor common code out of the container brokers.
	machineId := args.MachineConfig.MachineId
	lxcLogger.Infof("starting lxc container for machineId: %s", machineId)

	// Default to using the host network until we can configure.
	bridgeDevice := broker.agentConfig.Value(agent.LxcBridge)
	if bridgeDevice == "" {
		bridgeDevice = lxc.DefaultLxcBridge
	}
	network := container.BridgeNetworkConfig(bridgeDevice)

	series := args.Tools.OneSeries()
	args.MachineConfig.MachineContainerType = instance.LXC
	args.MachineConfig.Tools = args.Tools[0]

	config, err := broker.api.ContainerConfig()
	if err != nil {
		lxcLogger.Errorf("failed to get container config: %v", err)
		return nil, nil, nil, err
	}
	if err := environs.PopulateMachineConfig(
		args.MachineConfig,
		config.ProviderType,
		config.AuthorizedKeys,
		config.SSLHostnameVerification,
		config.Proxy,
		config.AptProxy,
	); err != nil {
		lxcLogger.Errorf("failed to populate machine config: %v", err)
		return nil, nil, nil, err
	}

	inst, hardware, err := broker.manager.CreateContainer(args.MachineConfig, series, network)
	if err != nil {
		lxcLogger.Errorf("failed to start container: %v", err)
		return nil, nil, nil, err
	}
	lxcLogger.Infof("started lxc container for machineId: %s, %s, %s", machineId, inst.Id(), hardware.String())
	return inst, hardware, nil, nil
}

// StopInstances shuts down the given instances.
func (broker *lxcBroker) StopInstances(ids ...instance.Id) error {
	// TODO: potentially parallelise.
	for _, id := range ids {
		lxcLogger.Infof("stopping lxc container for instance: %s", id)
		if err := broker.manager.DestroyContainer(id); err != nil {
			lxcLogger.Errorf("container did not stop: %v", err)
			return err
		}
	}
	return nil
}

// AllInstances only returns running containers.
func (broker *lxcBroker) AllInstances() (result []instance.Instance, err error) {
	return broker.manager.ListContainers()
}
