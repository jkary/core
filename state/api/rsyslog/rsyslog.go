// Copyright 2014 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package rsyslog

import (
	"fmt"

	"launchpad.net/juju-core/instance"
	"launchpad.net/juju-core/state/api/base"
	"launchpad.net/juju-core/state/api/params"
	"launchpad.net/juju-core/state/api/watcher"
)

const rsyslogAPI = "Rsyslog"

// RsyslogConfig holds the values needed for the rsyslog worker
type RsyslogConfig struct {
	CACert    string
	Port      int
	HostPorts []instance.HostPort
}

// State provides access to the Rsyslog API facade.
type State struct {
	caller base.Caller
}

// NewState creates a new client-side Rsyslog facade.
func NewState(caller base.Caller) *State {
	return &State{caller: caller}
}

// SetRsyslogCert sets the rsyslog CA certificate,
// which is used by clients to verify the server's
// identity and establish a TLS session.
func (st *State) SetRsyslogCert(caCert string) error {
	var result params.ErrorResult
	args := params.SetRsyslogCertParams{
		CACert: []byte(caCert),
	}
	err := st.caller.Call(rsyslogAPI, "", "SetRsyslogCert", args, &result)
	if err != nil {
		return err
	}
	if result.Error != nil {
		return result.Error
	}
	return nil
}

// WatchForChanges ...
func (st *State) WatchForRsyslogChanges(agentTag string) (watcher.NotifyWatcher, error) {
	var results params.NotifyWatchResults
	args := params.Entities{
		Entities: []params.Entity{{Tag: agentTag}},
	}
	fmt.Printf("tag: %s", agentTag)

	err := st.caller.Call(rsyslogAPI, "", "WatchForRsyslogChanges", args, &results)
	if err != nil {
		// TODO: Not directly tested
		return nil, err
	}
	if len(results.Results) != 1 {
		// TODO: Not directly tested
		return nil, fmt.Errorf("expected 1 result, got %d", len(results.Results))
	}
	result := results.Results[0]
	if result.Error != nil {
		//  TODO: Not directly tested
		return nil, result.Error
	}
	w := watcher.NewNotifyWatcher(st.caller, result)
	return w, nil
}

// GetRsyslogConfig returns a RsyslogConfig.
func (st *State) GetRsyslogConfig() (*RsyslogConfig, error) {
	var result params.RsyslogConfigResult
	err := st.caller.Call(rsyslogAPI, "", "GetRsyslogConfig", nil, &result)
	if err != nil {
		return nil, err
	}
	return &RsyslogConfig{
		CACert:    result.CACert,
		Port:      result.Port,
		HostPorts: result.HostPorts,
	}, nil
}
