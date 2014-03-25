// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package local

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"

	"launchpad.net/juju-core/agent/mongo"
	"launchpad.net/juju-core/container/kvm"
	"launchpad.net/juju-core/instance"
	"launchpad.net/juju-core/utils"
	"launchpad.net/juju-core/version"
)

var notLinuxError = errors.New("The local provider is currently only available for Linux")

const installMongodUbuntu = "MongoDB server must be installed to enable the local provider:"
const aptAddRepositoryJujuStable = `
    sudo apt-add-repository ppa:juju/stable   # required for MongoDB SSL support
    sudo apt-get update`
const aptGetInstallMongodbServer = `
    sudo apt-get install mongodb-server`

const installMongodGeneric = `
MongoDB server must be installed to enable the local provider.
Please consult your operating system distribution's documentation
for instructions on installing the MongoDB server. Juju requires
a MongoDB server built with SSL support.
`

const installLxcUbuntu = `
Linux Containers (LXC) userspace tools must be
installed to enable the local provider:

    sudo apt-get install lxc`

const installRsyslogGnutlsUbuntu = `
rsyslog-gnutls must be installed to enable the local provider:

    sudo apt-get install rsyslog-gnutls`

const installRsyslogGnutlsGeneric = `
rsyslog-gnutls must be installed to enable the local provider.
Please consult your operating system distribution's documentation
for instructions on installing this package.`

const installLxcGeneric = `
Linux Containers (LXC) userspace tools must be installed to enable the
local provider. Please consult your operating system distribution's
documentation for instructions on installing the LXC userspace tools.`

const errUnsupportedOS = `Unsupported operating system: %s
The local provider is currently only available for Linux`

// lowestMongoVersion is the lowest version of mongo that juju supports.
var lowestMongoVersion = version.Number{Major: 2, Minor: 2, Patch: 4}

// lxclsPath is the path to "lxc-ls", an LXC userspace tool
// we check the presence of to determine whether the
// tools are installed. This is a variable only to support
// unit testing.
var lxclsPath = "lxc-ls"

// isPackageInstalled is a variable to support testing.
var isPackageInstalled = utils.IsPackageInstalled

// defaultRsyslogGnutlsPath is the default path to the
// rsyslog GnuTLS module. This is a variable only to
// support unit testing.
var defaultRsyslogGnutlsPath = "/usr/lib/rsyslog/lmnsd_gtls.so"

// The operating system the process is running in.
// This is a variable only to support unit testing.
var goos = runtime.GOOS

// This is the regex for processing the results of mongod --verison
var mongoVerRegex = regexp.MustCompile(`db version v(\d+\.\d+\.\d+)`)

// VerifyPrerequisites verifies the prerequisites of
// the local machine (machine 0) for running the local
// provider.
var VerifyPrerequisites = func(containerType instance.ContainerType) error {
	if goos != "linux" {
		return fmt.Errorf(errUnsupportedOS, goos)
	}
	if err := verifyMongod(); err != nil {
		return err
	}
	if err := verifyRsyslogGnutls(); err != nil {
		return err
	}
	switch containerType {
	case instance.LXC:
		return verifyLxc()
	case instance.KVM:
		return kvm.VerifyKVMEnabled()
	}
	return fmt.Errorf("Unknown container type specified in the config.")
}

func verifyMongod() error {
	path, err := mongo.MongodPath()
	if err != nil {
		return wrapMongodNotExist(err)
	}

	ver, err := mongodVersion(path)
	if err != nil {
		return err
	}
	if ver.Compare(lowestMongoVersion) < 0 {
		return fmt.Errorf("installed version of mongod (%v) is not supported by Juju. "+
			"Juju requires version %v or greater.",
			ver,
			lowestMongoVersion)
	}
	return nil
}

func mongodVersion(path string) (version.Number, error) {
	data, err := utils.RunCommand(path, "--version")
	if err != nil {
		return version.Zero, wrapMongodNotExist(err)
	}

	return parseVersion(data)
}

func parseVersion(data string) (version.Number, error) {
	matches := mongoVerRegex.FindStringSubmatch(data)
	if len(matches) < 2 {
		return version.Zero, errors.New("could not parse mongod version")
	}
	return version.Parse(matches[1])
}

func verifyLxc() error {
	_, err := exec.LookPath(lxclsPath)
	if err != nil {
		return wrapLxcNotFound(err)
	}
	return nil
}

func verifyRsyslogGnutls() error {
	if isPackageInstalled("rsyslog-gnutls") {
		return nil
	}
	if utils.IsUbuntu() {
		return errors.New(installRsyslogGnutlsUbuntu)
	}
	// Not all Linuxes will distribute the module
	// in the same way. Check if it's in the default
	// location too.
	_, err := os.Stat(defaultRsyslogGnutlsPath)
	if err == nil {
		return nil
	}
	return fmt.Errorf("%v\n%s", err, installRsyslogGnutlsGeneric)
}

func wrapMongodNotExist(err error) error {
	if utils.IsUbuntu() {
		series := version.Current.Series
		args := []interface{}{err, installMongodUbuntu}
		format := "%v\n%s\n%s"
		if series == "precise" || series == "quantal" {
			format += "%s"
			args = append(args, aptAddRepositoryJujuStable)
		}
		args = append(args, aptGetInstallMongodbServer)
		return fmt.Errorf(format, args...)
	}
	return fmt.Errorf("%v\n%s", err, installMongodGeneric)
}

func wrapLxcNotFound(err error) error {
	if utils.IsUbuntu() {
		return fmt.Errorf("%v\n%s", err, installLxcUbuntu)
	}
	return fmt.Errorf("%v\n%s", err, installLxcGeneric)
}
