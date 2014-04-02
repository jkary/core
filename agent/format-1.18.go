// Copyright 2014 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package agent

import (
	"fmt"
	"net"
	"strconv"

	"launchpad.net/goyaml"
	"launchpad.net/juju-core/state/api/params"
	"launchpad.net/juju-core/version"
)

var format_1_18 = formatter_1_18{}

// formatter_1_18 is the formatter for the 1.18 format.
type formatter_1_18 struct {
}

// Ensure that the formatter_1_18 struct implements the formatter interface.
var _ formatter = formatter_1_18{}

// format_1_18Serialization holds information for a given agent.
type format_1_18Serialization struct {
	Tag               string
	DataDir           string
	LogDir            string
	Nonce             string
	Jobs              []params.MachineJob `yaml:",omitempty"`
	UpgradedToVersion *version.Number     `yaml:"upgradedToVersion"`

	CACert         string
	StateAddresses []string `yaml:",omitempty"`
	StatePassword  string   `yaml:",omitempty"`

	APIAddresses []string `yaml:",omitempty"`
	APIPassword  string   `yaml:",omitempty"`

	OldPassword string
	Values      map[string]string

	// Only state server machines have these next three items
	StateServerCert string `yaml:",omitempty"`
	StateServerKey  string `yaml:",omitempty"`
	APIPort         int    `yaml:",omitempty"`
	StatePort       int    `yaml:",omitempty"`
}

func init() {
	registerFormat(format_1_18)
}

func (formatter_1_18) version() string {
	return "1.18"
}

func (formatter_1_18) unmarshal(data []byte) (*configInternal, error) {
	// NOTE: this needs to handle the absence of StatePort and get it from the
	// address
	var format format_1_18Serialization
	if err := goyaml.Unmarshal(data, &format); err != nil {
		return nil, err
	}
	if format.UpgradedToVersion == nil || *format.UpgradedToVersion == version.Zero {
		// Assume we upgrade from 1.16.
		upgradedToVersion := version.MustParse("1.16.0")
		format.UpgradedToVersion = &upgradedToVersion
	}
	config := &configInternal{
		tag:               format.Tag,
		dataDir:           format.DataDir,
		logDir:            format.LogDir,
		jobs:              format.Jobs,
		upgradedToVersion: *format.UpgradedToVersion,
		nonce:             format.Nonce,
		caCert:            []byte(format.CACert),
		oldPassword:       format.OldPassword,
		servingInfo: params.StateServingInfo{
			Cert:       format.StateServerCert,
			PrivateKey: format.StateServerKey,
			APIPort:    format.APIPort,
			StatePort:  format.StatePort},
		values: format.Values,
	}
	if config.logDir == "" {
		config.logDir = DefaultLogDir
	}
	if config.dataDir == "" {
		config.dataDir = DefaultDataDir
	}
	if len(format.StateAddresses) > 0 {
		config.stateDetails = &connectionDetails{
			format.StateAddresses,
			format.StatePassword,
		}
	}
	if len(format.APIAddresses) > 0 {
		config.apiDetails = &connectionDetails{
			format.APIAddresses,
			format.APIPassword,
		}
	}
	if len(config.servingInfo.PrivateKey) != 0 {
		// There's a private key, then we need the state port,
		// which wasn't always in the  1.18 format. If it's not present
		// we can infer it from the ports in the state addresses.
		if config.servingInfo.StatePort == 0 && len(format.StateAddresses) > 0 {
			_, portString, err := net.SplitHostPort(format.StateAddresses[0])
			if err != nil {
				return nil, err
			}
			statePort, err := strconv.Atoi(portString)
			if err != nil {
				return nil, err
			}
			config.servingInfo.StatePort = statePort
		} else if config.servingInfo.StatePort == 0 {
			return nil, fmt.Errorf("server key found but no state port")
		}
	}
	return config, nil
}

func (formatter_1_18) marshal(config *configInternal) ([]byte, error) {
	format := &format_1_18Serialization{
		Tag:               config.tag,
		DataDir:           config.dataDir,
		LogDir:            config.logDir,
		Jobs:              config.jobs,
		UpgradedToVersion: &config.upgradedToVersion,
		Nonce:             config.nonce,
		CACert:            string(config.caCert),
		OldPassword:       config.oldPassword,
		StateServerCert:   config.servingInfo.Cert,
		StateServerKey:    config.servingInfo.PrivateKey,
		APIPort:           config.servingInfo.APIPort,
		Values:            config.values,
		StatePort:         config.servingInfo.StatePort,
	}
	if config.stateDetails != nil {
		format.StateAddresses = config.stateDetails.addresses
		format.StatePassword = config.stateDetails.password
	}
	if config.apiDetails != nil {
		format.APIAddresses = config.apiDetails.addresses
		format.APIPassword = config.apiDetails.password
	}
	return goyaml.Marshal(format)
}
