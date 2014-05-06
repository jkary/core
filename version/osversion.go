// Copyright 2014 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package version

import (
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/juju/loggo"
)

var logger = loggo.GetLogger("juju.version")

func readSeries(releaseFile string) string {
	data, err := ioutil.ReadFile(releaseFile)
	if err != nil {
		// Failed to read the LSB Release file, so fall back to OS probing
		return "unknown"
	}
	for _, line := range strings.Split(string(data), "\n") {
		const prefix = "DISTRIB_CODENAME="
		if strings.HasPrefix(line, prefix) {
			return strings.Trim(line[len(prefix):], "\t '\"")
		}
	}
	return "unknown"
}

type kernelVersionFunc func() (string, error)

// kernelToMajor takes a dotted version and returns just the Major portion
func kernelToMajor(getKernelVersion kernelVersionFunc) (int, error) {
	fullVersion, err := getKernelVersion()
	if err != nil {
		return 0, err
	}
	parts := strings.SplitN(fullVersion, ".", 2)
	majorVersion, err := strconv.ParseInt(parts[0], 10, 32)
	if err != nil {
		return 0, err
	}
	return int(majorVersion), nil
}

func darwinVersionFromKernelVersion(getKernelVersion kernelVersionFunc) string {
	majorVersion, err := kernelToMajor(getKernelVersion)
	if err != nil {
		logger.Infof("unable to determine OS version: %v", err)
		return "unknown"
	}
	return darwinSeriesFromMajorVersion(majorVersion)
}

// TODO(jam): 2014-05-06 bug #XXXXX
// should we have a system file that we can read so this can be updated without
// recompiling Juju?
var darwinVersions = map[int]string{
	13: "mavericks",
	12: "mountainlion",
	11: "lion",
	10: "snowleopard",
	9:  "leopard",
	8:  "tiger",
	7:  "panther",
	6:  "jaguar",
	5:  "puma",
}

func darwinSeriesFromMajorVersion(majorVersion int) string {
	if series, ok := darwinVersions[majorVersion]; ok {
		return series
	}
	return "unknown"
}
