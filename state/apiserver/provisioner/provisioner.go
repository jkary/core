// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provisioner

import (
	"launchpad.net/juju-core/constraints"
	"launchpad.net/juju-core/instance"
	"launchpad.net/juju-core/names"
	"launchpad.net/juju-core/state"
	"launchpad.net/juju-core/state/api/params"
	"launchpad.net/juju-core/state/apiserver/common"
	"launchpad.net/juju-core/state/watcher"
	"launchpad.net/juju-core/utils/set"
)

// ProvisionerAPI provides access to the Provisioner API facade.
type ProvisionerAPI struct {
	*common.Remover
	*common.StatusSetter
	*common.DeadEnsurer
	*common.PasswordChanger
	*common.LifeGetter
	*common.StateAddresser
	*common.APIAddresser
	*common.ToolsGetter
	*common.EnvironWatcher
	*common.EnvironMachinesWatcher
	*common.InstanceIdGetter

	st                  *state.State
	resources           *common.Resources
	authorizer          common.Authorizer
	getAuthFunc         common.GetAuthFunc
	getCanWatchMachines common.GetAuthFunc
}

// NewProvisionerAPI creates a new server-side ProvisionerAPI facade.
func NewProvisionerAPI(
	st *state.State,
	resources *common.Resources,
	authorizer common.Authorizer,
) (*ProvisionerAPI, error) {
	if !authorizer.AuthMachineAgent() && !authorizer.AuthEnvironManager() {
		return nil, common.ErrPerm
	}
	getAuthFunc := func() (common.AuthFunc, error) {
		isEnvironManager := authorizer.AuthEnvironManager()
		isMachineAgent := authorizer.AuthMachineAgent()
		authEntityTag := authorizer.GetAuthTag()

		return func(tag string) bool {
			if isMachineAgent && tag == authEntityTag {
				// A machine agent can always access its own machine.
				return true
			}
			_, id, err := names.ParseTag(tag, names.MachineTagKind)
			if err != nil {
				return false
			}
			parentId := state.ParentId(id)
			if parentId == "" {
				// All top-level machines are accessible by the
				// environment manager.
				return isEnvironManager
			}
			// All containers with the authenticated machine as a
			// parent are accessible by it.
			return isMachineAgent && names.MachineTag(parentId) == authEntityTag
		}, nil
	}
	// Both provisioner types can watch the environment.
	getCanWatch := common.AuthAlways(true)
	// Only the environment provisioner can read secrets.
	getCanReadSecrets := common.AuthAlways(authorizer.AuthEnvironManager())
	return &ProvisionerAPI{
		Remover:                common.NewRemover(st, false, getAuthFunc),
		StatusSetter:           common.NewStatusSetter(st, getAuthFunc),
		DeadEnsurer:            common.NewDeadEnsurer(st, getAuthFunc),
		PasswordChanger:        common.NewPasswordChanger(st, getAuthFunc),
		LifeGetter:             common.NewLifeGetter(st, getAuthFunc),
		StateAddresser:         common.NewStateAddresser(st),
		APIAddresser:           common.NewAPIAddresser(st, resources),
		ToolsGetter:            common.NewToolsGetter(st, getAuthFunc),
		EnvironWatcher:         common.NewEnvironWatcher(st, resources, getCanWatch, getCanReadSecrets),
		EnvironMachinesWatcher: common.NewEnvironMachinesWatcher(st, resources, getCanReadSecrets),
		InstanceIdGetter:       common.NewInstanceIdGetter(st, getAuthFunc),
		st:                     st,
		resources:              resources,
		authorizer:             authorizer,
		getAuthFunc:            getAuthFunc,
		getCanWatchMachines:    getCanReadSecrets,
	}, nil
}

func (p *ProvisionerAPI) getMachine(canAccess common.AuthFunc, tag string) (*state.Machine, error) {
	if !canAccess(tag) {
		return nil, common.ErrPerm
	}
	entity, err := p.st.FindEntity(tag)
	if err != nil {
		return nil, err
	}
	// The authorization function guarantees that the tag represents a
	// machine.
	return entity.(*state.Machine), nil
}

func (p *ProvisionerAPI) watchOneMachineContainers(arg params.WatchContainer) (params.StringsWatchResult, error) {
	nothing := params.StringsWatchResult{}
	canAccess, err := p.getAuthFunc()
	if err != nil {
		return nothing, err
	}
	if !canAccess(arg.MachineTag) {
		return nothing, common.ErrPerm
	}
	_, id, err := names.ParseTag(arg.MachineTag, names.MachineTagKind)
	if err != nil {
		return nothing, err
	}
	machine, err := p.st.Machine(id)
	if err != nil {
		return nothing, err
	}
	var watch state.StringsWatcher
	if arg.ContainerType != "" {
		watch = machine.WatchContainers(instance.ContainerType(arg.ContainerType))
	} else {
		watch = machine.WatchAllContainers()
	}
	// Consume the initial event and forward it to the result.
	if changes, ok := <-watch.Changes(); ok {
		return params.StringsWatchResult{
			StringsWatcherId: p.resources.Register(watch),
			Changes:          changes,
		}, nil
	}
	return nothing, watcher.MustErr(watch)
}

// WatchContainers starts a StringsWatcher to watch containers deployed to
// any machine passed in args.
func (p *ProvisionerAPI) WatchContainers(args params.WatchContainers) (params.StringsWatchResults, error) {
	result := params.StringsWatchResults{
		Results: make([]params.StringsWatchResult, len(args.Params)),
	}
	for i, arg := range args.Params {
		watcherResult, err := p.watchOneMachineContainers(arg)
		result.Results[i] = watcherResult
		result.Results[i].Error = common.ServerError(err)
	}
	return result, nil
}

// WatchAllContainers starts a StringsWatcher to watch all containers deployed to
// any machine passed in args.
func (p *ProvisionerAPI) WatchAllContainers(args params.WatchContainers) (params.StringsWatchResults, error) {
	return p.WatchContainers(args)
}

// SetSupportedContainers updates the list of containers supported by the machines passed in args.
func (p *ProvisionerAPI) SetSupportedContainers(
	args params.MachineContainersParams) (params.ErrorResults, error) {

	result := params.ErrorResults{
		Results: make([]params.ErrorResult, len(args.Params)),
	}
	for i, arg := range args.Params {
		canAccess, err := p.getAuthFunc()
		if err != nil {
			return result, err
		}
		machine, err := p.getMachine(canAccess, arg.MachineTag)
		if err != nil {
			result.Results[i].Error = common.ServerError(err)
			continue
		}
		if len(arg.ContainerTypes) == 0 {
			err = machine.SupportsNoContainers()
		} else {
			err = machine.SetSupportedContainers(arg.ContainerTypes)
		}
		if err != nil {
			result.Results[i].Error = common.ServerError(err)
		}
	}
	return result, nil
}

// ContainerConfig returns information from the environment config that are
// needed for container cloud-init.
func (p *ProvisionerAPI) ContainerConfig() (params.ContainerConfig, error) {
	result := params.ContainerConfig{}
	config, err := p.st.EnvironConfig()
	if err != nil {
		return result, err
	}
	result.ProviderType = config.Type()
	result.AuthorizedKeys = config.AuthorizedKeys()
	result.SSLHostnameVerification = config.SSLHostnameVerification()
	result.Proxy = config.ProxySettings()
	result.AptProxy = config.AptProxySettings()
	return result, nil
}

// Status returns the status of each given machine entity.
func (p *ProvisionerAPI) Status(args params.Entities) (params.StatusResults, error) {
	result := params.StatusResults{
		Results: make([]params.StatusResult, len(args.Entities)),
	}
	canAccess, err := p.getAuthFunc()
	if err != nil {
		return result, err
	}
	for i, entity := range args.Entities {
		machine, err := p.getMachine(canAccess, entity.Tag)
		if err == nil {
			r := &result.Results[i]
			r.Status, r.Info, r.Data, err = machine.Status()
		}
		result.Results[i].Error = common.ServerError(err)
	}
	return result, nil
}

// MachinesWithTransientErrors returns status data for machines with provisioning
// errors which are transient.
func (p *ProvisionerAPI) MachinesWithTransientErrors() (params.StatusResults, error) {
	results := params.StatusResults{}
	canAccessFunc, err := p.getAuthFunc()
	if err != nil {
		return results, err
	}
	// TODO (wallyworld) - add state.State API for more efficient machines query
	machines, err := p.st.AllMachines()
	if err != nil {
		return results, err
	}
	for _, machine := range machines {
		if !canAccessFunc(machine.Tag()) {
			continue
		}
		if _, provisionedErr := machine.InstanceId(); provisionedErr == nil {
			// Machine may have been provisioned but machiner hasn't set the
			// status to Started yet.
			continue
		}
		result := params.StatusResult{}
		if result.Status, result.Info, result.Data, err = machine.Status(); err != nil {
			continue
		}
		if result.Status != params.StatusError {
			continue
		}
		// Transient errors are marked as such in the status data.
		if transient, ok := result.Data["transient"].(bool); !ok || !transient {
			continue
		}
		result.Id = machine.Id()
		result.Life = params.Life(machine.Life().String())
		results.Results = append(results.Results, result)
	}
	return results, nil
}

// Series returns the deployed series for each given machine entity.
func (p *ProvisionerAPI) Series(args params.Entities) (params.StringResults, error) {
	result := params.StringResults{
		Results: make([]params.StringResult, len(args.Entities)),
	}
	canAccess, err := p.getAuthFunc()
	if err != nil {
		return result, err
	}
	for i, entity := range args.Entities {
		machine, err := p.getMachine(canAccess, entity.Tag)
		if err == nil {
			result.Results[i].Result = machine.Series()
		}
		result.Results[i].Error = common.ServerError(err)
	}
	return result, nil
}

// DistributionGroup returns, for each given machine entity,
// a slice of instance.Ids that belong to the same distribution
// group as that machine. This information may be used to
// distribute instances for high availability.
func (p *ProvisionerAPI) DistributionGroup(args params.Entities) (params.DistributionGroupResults, error) {
	result := params.DistributionGroupResults{
		Results: make([]params.DistributionGroupResult, len(args.Entities)),
	}
	canAccess, err := p.getAuthFunc()
	if err != nil {
		return result, err
	}
	for i, entity := range args.Entities {
		machine, err := p.getMachine(canAccess, entity.Tag)
		if err == nil {
			// If the machine is an environment manager, return
			// environment manager instances. Otherwise, return
			// instances with services in common with the machine
			// being provisioned.
			if machine.IsManager() {
				result.Results[i].Result, err = environManagerInstances(p.st)
			} else {
				result.Results[i].Result, err = commonServiceInstances(p.st, machine)
			}
		}
		result.Results[i].Error = common.ServerError(err)
	}
	return result, nil
}

// environManagerInstances returns all environ manager instances.
func environManagerInstances(st *state.State) ([]instance.Id, error) {
	info, err := st.StateServerInfo()
	if err != nil {
		return nil, err
	}
	instances := make([]instance.Id, 0, len(info.MachineIds))
	for _, id := range info.MachineIds {
		machine, err := st.Machine(id)
		if err != nil {
			return nil, err
		}
		instanceId, err := machine.InstanceId()
		if err == nil {
			instances = append(instances, instanceId)
		} else if !state.IsNotProvisionedError(err) {
			return nil, err
		}
	}
	return instances, nil
}

// commonServiceInstances returns instances with
// services in common with the specified machine.
func commonServiceInstances(st *state.State, m *state.Machine) ([]instance.Id, error) {
	units, err := m.Units()
	if err != nil {
		return nil, err
	}
	var instanceIdSet set.Strings
	for _, unit := range units {
		if !unit.IsPrincipal() {
			continue
		}
		service, err := unit.Service()
		if err != nil {
			return nil, err
		}
		allUnits, err := service.AllUnits()
		if err != nil {
			return nil, err
		}
		for _, unit := range allUnits {
			machineId, err := unit.AssignedMachineId()
			if state.IsNotAssigned(err) {
				continue
			} else if err != nil {
				return nil, err
			}
			machine, err := st.Machine(machineId)
			if err != nil {
				return nil, err
			}
			instanceId, err := machine.InstanceId()
			if err == nil {
				instanceIdSet.Add(string(instanceId))
			} else if state.IsNotProvisionedError(err) {
				continue
			} else {
				return nil, err
			}
		}
	}
	instanceIds := make([]instance.Id, instanceIdSet.Size())
	// Sort values to simplify testing.
	for i, instanceId := range instanceIdSet.SortedValues() {
		instanceIds[i] = instance.Id(instanceId)
	}
	return instanceIds, nil
}

// Constraints returns the constraints for each given machine entity.
func (p *ProvisionerAPI) Constraints(args params.Entities) (params.ConstraintsResults, error) {
	result := params.ConstraintsResults{
		Results: make([]params.ConstraintsResult, len(args.Entities)),
	}
	canAccess, err := p.getAuthFunc()
	if err != nil {
		return result, err
	}
	for i, entity := range args.Entities {
		machine, err := p.getMachine(canAccess, entity.Tag)
		if err == nil {
			var cons constraints.Value
			cons, err = machine.Constraints()
			if err == nil {
				result.Results[i].Constraints = cons
			}
		}
		result.Results[i].Error = common.ServerError(err)
	}
	return result, nil
}

// RequestedNetworks returns the requested networks for each given machine entity.
func (p *ProvisionerAPI) RequestedNetworks(args params.Entities) (params.NetworksResults, error) {
	result := params.NetworksResults{
		Results: make([]params.NetworkResult, len(args.Entities)),
	}
	canAccess, err := p.getAuthFunc()
	if err != nil {
		return result, err
	}
	for i, entity := range args.Entities {
		machine, err := p.getMachine(canAccess, entity.Tag)
		if err == nil {
			var includeNetworks []string
			var excludeNetworks []string
			includeNetworks, excludeNetworks, err = machine.RequestedNetworks()
			if err == nil {
				result.Results[i].IncludeNetworks = includeNetworks
				result.Results[i].ExcludeNetworks = excludeNetworks
			}
		}
		result.Results[i].Error = common.ServerError(err)
	}
	return result, nil
}

// AddNetwork creates one or more new networks with the given
// parameters. Only the environment manager can add networks. If any
// of the given networks already exists, an error satisfying
// params.IsCodeAlreadyExists will be returned for it.
func (p *ProvisionerAPI) AddNetwork(args params.AddNetworkParams) (params.ErrorResults, error) {
	result := params.ErrorResults{
		Results: make([]params.ErrorResult, len(args.Networks)),
	}
	if !p.authorizer.AuthEnvironManager() {
		return result, common.ErrPerm
	}
	for i, arg := range args.Networks {
		_, err := p.st.AddNetwork(arg.Name, arg.CIDR, arg.VLANTag)
		result.Results[i].Error = common.ServerError(err)
	}
	return result, nil
}

// AddNetworkInterface creates one or more new network interfaces with
// the given parameters. If any of the given interfaces already exist,
// an error satisfying params.IsCodeAlreadyExists will be returned for
// it.
func (p *ProvisionerAPI) AddNetworkInterface(args params.AddNetworkInterfaceParams) (params.ErrorResults, error) {
	result := params.ErrorResults{
		Results: make([]params.ErrorResult, len(args.Interfaces)),
	}
	canAccess, err := p.getAuthFunc()
	if err != nil {
		return result, err
	}
	for i, arg := range args.Interfaces {
		machine, err := p.getMachine(canAccess, arg.MachineTag)
		if err == nil {
			_, err = machine.AddNetworkInterface(arg.MACAddress, arg.InterfaceName, arg.NetworkName)
		}
		result.Results[i].Error = common.ServerError(err)
	}
	return result, nil
}

// SetProvisioned sets the provider specific machine id, nonce and
// metadata for each given machine. Once set, the instance id cannot
// be changed.
func (p *ProvisionerAPI) SetProvisioned(args params.SetProvisioned) (params.ErrorResults, error) {
	result := params.ErrorResults{
		Results: make([]params.ErrorResult, len(args.Machines)),
	}
	canAccess, err := p.getAuthFunc()
	if err != nil {
		return result, err
	}
	for i, arg := range args.Machines {
		machine, err := p.getMachine(canAccess, arg.Tag)
		if err == nil {
			err = machine.SetProvisioned(arg.InstanceId, arg.Nonce, arg.Characteristics)
		}
		result.Results[i].Error = common.ServerError(err)
	}
	return result, nil
}

// WatchMachineErrorRetry returns a NotifyWatcher that notifies when
// the provisioner should retry provisioning machines with transient errors.
func (p *ProvisionerAPI) WatchMachineErrorRetry() (params.NotifyWatchResult, error) {
	result := params.NotifyWatchResult{}
	canWatch, err := p.getCanWatchMachines()
	if err != nil {
		return params.NotifyWatchResult{}, err
	}
	if !canWatch("") {
		return result, common.ErrPerm
	}
	watch := newWatchMachineErrorRetry()
	// Consume any initial event and forward it to the result.
	if _, ok := <-watch.Changes(); ok {
		result.NotifyWatcherId = p.resources.Register(watch)
	} else {
		return result, watcher.MustErr(watch)
	}
	return result, nil
}
