package maas

import (
	"encoding/base64"
	"errors"
	"fmt"
	"launchpad.net/gomaasapi"
	"launchpad.net/juju-core/constraints"
	"launchpad.net/juju-core/environs"
	"launchpad.net/juju-core/environs/cloudinit"
	"launchpad.net/juju-core/environs/config"
	"launchpad.net/juju-core/log"
	"launchpad.net/juju-core/state"
	"launchpad.net/juju-core/state/api"
	"launchpad.net/juju-core/state/api/params"
	"launchpad.net/juju-core/trivial"
	"launchpad.net/juju-core/version"
	"net/url"
	"sync"
	"time"
)

const (
	mgoPort     = 37017
	apiPort     = 17070
	jujuDataDir = "/var/lib/juju"
	// We're using v1.0 of the MAAS API.
	apiVersion = "1.0"
)

var mgoPortSuffix = fmt.Sprintf(":%d", mgoPort)
var apiPortSuffix = fmt.Sprintf(":%d", apiPort)

var longAttempt = trivial.AttemptStrategy{
	Total: 3 * time.Minute,
	Delay: 1 * time.Second,
}

type maasEnviron struct {
	name string

	// ecfgMutex protects the *Unlocked fields below.
	ecfgMutex sync.Mutex

	ecfgUnlocked       *maasEnvironConfig
	maasClientUnlocked *gomaasapi.MAASObject
	storageUnlocked    environs.Storage
}

var _ environs.Environ = (*maasEnviron)(nil)

var couldNotAllocate = errors.New("Could not allocate MAAS environment object.")

func NewEnviron(cfg *config.Config) (*maasEnviron, error) {
	env := new(maasEnviron)
	if env == nil {
		return nil, couldNotAllocate
	}
	err := env.SetConfig(cfg)
	if err != nil {
		return nil, err
	}
	env.storageUnlocked = NewStorage(env)
	return env, nil
}

func (env *maasEnviron) Name() string {
	return env.name
}

// TODO: this code is cargo-culted from the openstack/ec2 providers.
func (env *maasEnviron) findTools() (*state.Tools, error) {
	flags := environs.HighestVersion | environs.CompatVersion
	v := version.Current
	v.Series = env.Config().DefaultSeries()
	return environs.FindTools(env, v, flags)
}

// makeMachineConfig sets up a basic machine configuration for use with
// userData().  You may still need to supply more information, but this takes
// care of the fixed entries and the ones that are always needed.
func (env *maasEnviron) makeMachineConfig(machineID, machineNonce string, stateInfo *state.Info, apiInfo *api.Info, tools *state.Tools) *cloudinit.MachineConfig {
	return &cloudinit.MachineConfig{
		// Fixed entries.
		MongoPort: mgoPort,
		APIPort:   apiPort,
		DataDir:   jujuDataDir,

		// Entries based purely on what's in the environment.
		AuthorizedKeys: env.ecfg().AuthorizedKeys(),

		// Parameter entries.
		MachineId:    machineID,
		MachineNonce: machineNonce,
		StateInfo:    stateInfo,
		APIInfo:      apiInfo,
		Tools:        tools,
	}
}

// startBootstrapNode starts the juju bootstrap node for this environment.
func (env *maasEnviron) startBootstrapNode(tools *state.Tools, cert, key []byte, password string) (environs.Instance, error) {
	config, err := environs.BootstrapConfig(env.Provider(), env.Config(), tools)
	if err != nil {
		return nil, fmt.Errorf("unable to determine initial configuration: %v", err)
	}
	caCert, hasCert := env.Config().CACert()
	if !hasCert {
		return nil, fmt.Errorf("no CA certificate in environment configuration")
	}
	stateInfo := state.Info{
		Password: trivial.PasswordHash(password),
		CACert:   caCert,
	}
	apiInfo := api.Info{
		Password: trivial.PasswordHash(password),
		CACert:   caCert,
	}

	// The bootstrap instance gets machine id "0".  This is not related to
	// instance ids or MAAS system ids.  Juju assigns the machine ID.
	const machineID = "0"

	mcfg := env.makeMachineConfig(machineID, state.BootstrapNonce, &stateInfo, &apiInfo, tools)
	mcfg.StateServer = true
	mcfg.StateServerCert = cert
	mcfg.StateServerKey = key
	mcfg.Config = config

	inst, err := env.obtainNode(machineID, &stateInfo, &apiInfo, tools, mcfg)
	if err != nil {
		return nil, fmt.Errorf("cannot start bootstrap instance: %v", err)
	}
	return inst, nil
}

// Bootstrap is specified in the Environ interface.
func (env *maasEnviron) Bootstrap(cons constraints.Value, stateServerCert, stateServerKey []byte) error {
	constraints := cons.String()
	if constraints != "" {
		log.Warningf("ignoring constraints '%s' (not implemented)", constraints)
	}

	// This was all cargo-culted from the EC2 provider.
	password := env.Config().AdminSecret()
	if password == "" {
		return fmt.Errorf("admin-secret is required for bootstrap")
	}
	log.Debugf("environs/maas: bootstrapping environment %q.", env.Name())
	tools, err := env.findTools()
	if err != nil {
		return err
	}
	inst, err := env.startBootstrapNode(tools, stateServerCert, stateServerKey, password)
	if err != nil {
		return err
	}
	err = env.saveState(&bootstrapState{StateInstances: []state.InstanceId{inst.Id()}})
	if err != nil {
		env.releaseInstance(inst)
		return fmt.Errorf("cannot save state: %v", err)
	}

	// TODO make safe in the case of racing Bootstraps
	// If two Bootstraps are called concurrently, there's
	// no way to make sure that only one succeeds.

	return nil
}

// StateInfo is specified in the Environ interface.
func (env *maasEnviron) StateInfo() (*state.Info, *api.Info, error) {
	// This code is cargo-culted from the openstack/ec2 providers.
	// It's a bit unclear what the "longAttempt" loop is actually for
	// but this should probably be refactored outside of the provider
	// code.
	st, err := env.loadState()
	if err != nil {
		return nil, nil, err
	}
	cert, hasCert := env.Config().CACert()
	if !hasCert {
		return nil, nil, fmt.Errorf("no CA certificate in environment configuration")
	}
	var stateAddrs []string
	var apiAddrs []string
	// Wait for the DNS names of any of the instances
	// to become available.
	log.Debugf("environs/maas: waiting for DNS name(s) of state server instances %v", st.StateInstances)
	for a := longAttempt.Start(); len(stateAddrs) == 0 && a.Next(); {
		insts, err := env.Instances(st.StateInstances)
		if err != nil && err != environs.ErrPartialInstances {
			log.Debugf("error getting state instance: %v", err.Error())
			return nil, nil, err
		}
		log.Debugf("started processing instances: %#v", insts)
		for _, inst := range insts {
			if inst == nil {
				continue
			}
			name, err := inst.DNSName()
			if err != nil {
				continue
			}
			if name != "" {
				stateAddrs = append(stateAddrs, name+mgoPortSuffix)
				apiAddrs = append(apiAddrs, name+apiPortSuffix)
			}
		}
	}
	if len(stateAddrs) == 0 {
		return nil, nil, fmt.Errorf("timed out waiting for mgo address from %v", st.StateInstances)
	}
	return &state.Info{
			Addrs:  stateAddrs,
			CACert: cert,
		}, &api.Info{
			Addrs:  apiAddrs,
			CACert: cert,
		}, nil
}

// ecfg returns the environment's maasEnvironConfig, and protects it with a
// mutex.
func (env *maasEnviron) ecfg() *maasEnvironConfig {
	env.ecfgMutex.Lock()
	defer env.ecfgMutex.Unlock()
	return env.ecfgUnlocked
}

// Config is specified in the Environ interface.
func (env *maasEnviron) Config() *config.Config {
	return env.ecfg().Config
}

// SetConfig is specified in the Environ interface.
func (env *maasEnviron) SetConfig(cfg *config.Config) error {
	env.ecfgMutex.Lock()
	defer env.ecfgMutex.Unlock()

	// The new config has already been validated by itself, but now we
	// validate the transition from the old config to the new.
	var oldCfg *config.Config
	if env.ecfgUnlocked != nil {
		oldCfg = env.ecfgUnlocked.Config
	}
	cfg, err := env.Provider().Validate(cfg, oldCfg)
	if err != nil {
		return err
	}

	ecfg, err := providerInstance.newConfig(cfg)
	if err != nil {
		return err
	}

	env.name = cfg.Name()
	env.ecfgUnlocked = ecfg

	authClient, err := gomaasapi.NewAuthenticatedClient(ecfg.MAASServer(), ecfg.MAASOAuth(), apiVersion)
	if err != nil {
		return err
	}
	env.maasClientUnlocked = gomaasapi.NewMAAS(*authClient)

	return nil
}

// getMAASClient returns a MAAS client object to use for a request, in a
// lock-protected fashioon.
func (env *maasEnviron) getMAASClient() *gomaasapi.MAASObject {
	env.ecfgMutex.Lock()
	defer env.ecfgMutex.Unlock()

	return env.maasClientUnlocked
}

// acquireNode allocates a node from the MAAS.
func (environ *maasEnviron) acquireNode() (gomaasapi.MAASObject, error) {
	retry := trivial.AttemptStrategy{
		Total: 5 * time.Second,
		Delay: 200 * time.Millisecond,
	}
	var result gomaasapi.JSONObject
	var err error
	for a := retry.Start(); a.Next(); {
		client := environ.getMAASClient().GetSubObject("nodes/")
		result, err = client.CallPost("acquire", nil)
		if err == nil {
			break
		}
	}
	if err != nil {
		return gomaasapi.MAASObject{}, err
	}
	node, err := result.GetMAASObject()
	if err != nil {
		msg := fmt.Errorf("unexpected result from 'acquire' on MAAS API: %v", err)
		return gomaasapi.MAASObject{}, msg
	}
	return node, nil
}

// startNode installs and boots a node.
func (environ *maasEnviron) startNode(node gomaasapi.MAASObject, tools *state.Tools, userdata []byte) error {
	retry := trivial.AttemptStrategy{
		Total: 5 * time.Second,
		Delay: 200 * time.Millisecond,
	}
	userDataParam := base64.StdEncoding.EncodeToString(userdata)
	params := url.Values{
		"distro_series": {tools.Series},
		"user_data":     {userDataParam},
	}
	// Initialize err to a non-nil value as a sentinel for the following
	// loop.
	err := fmt.Errorf("(no error)")
	for a := retry.Start(); a.Next() && err != nil; {
		_, err = node.CallPost("start", params)
	}
	return err
}

// obtainNode allocates and starts a MAAS node.  It is used both for the
// implementation of StartInstance, and to initialize the bootstrap node.
func (environ *maasEnviron) obtainNode(machineId string, stateInfo *state.Info, apiInfo *api.Info, tools *state.Tools, mcfg *cloudinit.MachineConfig) (*maasInstance, error) {

	log.Debugf("environs/maas: starting machine %s in $q running tools version %q from %q", machineId, environ.name, tools.Binary, tools.URL)

	node, err := environ.acquireNode()
	if err != nil {
		return nil, fmt.Errorf("cannot run instances: %v", err)
	}

	hostname, err := node.GetField("hostname")
	if err != nil {
		return nil, err
	}
	instance := maasInstance{&node, environ}
	info := machineInfo{string(instance.Id()), hostname}
	runCmd, err := info.cloudinitRunCmd()
	if err != nil {
		return nil, err
	}
	userdata, err := userData(mcfg, runCmd)
	if err != nil {
		msg := fmt.Errorf("could not compose userdata for bootstrap node: %v", err)
		return nil, msg
	}
	err = environ.startNode(node, tools, userdata)
	if err != nil {
		environ.releaseInstance(&instance)
		return nil, fmt.Errorf("cannot start instance: %v", err)
	}
	log.Debugf("environs/maas: started instance %q", instance.Id())
	return &instance, nil
}

// StartInstance is specified in the Environ interface.
func (environ *maasEnviron) StartInstance(machineID, machineNonce string, series string, cons constraints.Value, stateInfo *state.Info, apiInfo *api.Info) (environs.Instance, error) {
	// TODO: Support series and constraints.  They were added to the
	// interface after we implemented.
	flags := environs.HighestVersion | environs.CompatVersion
	var err error
	tools, err := environs.FindTools(environ, version.Current, flags)
	if err != nil {
		return nil, err
	}

	mcfg := environ.makeMachineConfig(machineID, machineNonce, stateInfo, apiInfo, tools)
	return environ.obtainNode(machineID, stateInfo, apiInfo, tools, mcfg)
}

// StopInstances is specified in the Environ interface.
func (environ *maasEnviron) StopInstances(instances []environs.Instance) error {
	// Shortcut to exit quickly if 'instances' is an empty slice or nil.
	if len(instances) == 0 {
		return nil
	}
	// Tell MAAS to release each of the instances.  If there are errors,
	// return only the first one (but release all instances regardless).
	// Note that releasing instances also turns them off.
	var firstErr error
	for _, instance := range instances {
		err := environ.releaseInstance(instance)
		if firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// releaseInstance releases a single instance.
func (environ *maasEnviron) releaseInstance(inst environs.Instance) error {
	maasInst := inst.(*maasInstance)
	maasObj := maasInst.maasObject
	_, err := maasObj.CallPost("release", nil)
	if err != nil {
		log.Debugf("environs/maas: error releasing instance %v", maasInst)
	}
	return err
}

// Instances returns the environs.Instance objects corresponding to the given
// slice of state.InstanceId.  Similar to what the ec2 provider does,
// Instances returns nil if the given slice is empty or nil.
func (environ *maasEnviron) Instances(ids []state.InstanceId) ([]environs.Instance, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	return environ.instances(ids)
}

// instances is an internal method which returns the instances matching the
// given instance ids or all the instances if 'ids' is empty.
// If the some of the intances could not be found, it returns the instance
// that could be found plus the error environs.ErrPartialInstances in the error
// return.
func (environ *maasEnviron) instances(ids []state.InstanceId) ([]environs.Instance, error) {
	nodeListing := environ.getMAASClient().GetSubObject("nodes")
	filter := getSystemIdValues(ids)
	listNodeObjects, err := nodeListing.CallGet("list", filter)
	if err != nil {
		return nil, err
	}
	listNodes, err := listNodeObjects.GetArray()
	if err != nil {
		return nil, err
	}
	instances := make([]environs.Instance, len(listNodes))
	for index, nodeObj := range listNodes {
		node, err := nodeObj.GetMAASObject()
		if err != nil {
			return nil, err
		}
		instances[index] = &maasInstance{
			maasObject: &node,
			environ:    environ,
		}
	}
	if len(ids) != 0 && len(ids) != len(instances) {
		return instances, environs.ErrPartialInstances
	}
	return instances, nil
}

// AllInstances returns all the environs.Instance in this provider.
func (environ *maasEnviron) AllInstances() ([]environs.Instance, error) {
	return environ.instances(nil)
}

// Storage is defined by the Environ interface.
func (env *maasEnviron) Storage() environs.Storage {
	env.ecfgMutex.Lock()
	defer env.ecfgMutex.Unlock()
	return env.storageUnlocked
}

// PublicStorage is defined by the Environ interface.
func (env *maasEnviron) PublicStorage() environs.StorageReader {
	// MAAS does not have a shared storage.
	return environs.EmptyStorage
}

func (environ *maasEnviron) Destroy(ensureInsts []environs.Instance) error {
	log.Debugf("environs/maas: destroying environment %q", environ.name)
	insts, err := environ.AllInstances()
	if err != nil {
		return fmt.Errorf("cannot get instances: %v", err)
	}
	found := make(map[state.InstanceId]bool)
	for _, inst := range insts {
		found[inst.Id()] = true
	}

	// Add any instances we've been told about but haven't yet shown
	// up in the instance list.
	for _, inst := range ensureInsts {
		id := inst.Id()
		if !found[id] {
			insts = append(insts, inst)
			found[id] = true
		}
	}
	err = environ.StopInstances(insts)
	if err != nil {
		return err
	}

	// To properly observe e.storageUnlocked we need to get its value while
	// holding e.ecfgMutex. e.Storage() does this for us, then we convert
	// back to the (*storage) to access the private deleteAll() method.
	st := environ.Storage().(*maasStorage)
	return st.deleteAll()
}

func (*maasEnviron) AssignmentPolicy() state.AssignmentPolicy {
	return state.AssignUnused
}

// MAAS does not do firewalling so these port methods do nothing.
func (*maasEnviron) OpenPorts([]params.Port) error {
	log.Debugf("environs/maas: unimplemented OpenPorts() called")
	return nil
}

func (*maasEnviron) ClosePorts([]params.Port) error {
	log.Debugf("environs/maas: unimplemented ClosePorts() called")
	return nil
}

func (*maasEnviron) Ports() ([]params.Port, error) {
	log.Debugf("environs/maas: unimplemented Ports() called")
	return []params.Port{}, nil
}

func (*maasEnviron) Provider() environs.EnvironProvider {
	return &providerInstance
}