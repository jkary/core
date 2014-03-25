// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package ec2

import (
	"launchpad.net/goamz/aws"

	"launchpad.net/juju-core/environs/instances"
	"launchpad.net/juju-core/juju/arch"
)

// Type of virtualisation used.
var (
	paravirtual = "pv"
	hvm         = "hvm"
)

// all instance types can run amd64 images, and some can also run i386 ones.
var (
	amd64 = []string{arch.AMD64}
	both  = []string{arch.AMD64, arch.I386}
)

// allRegions is defined here to allow tests to override the content.
var allRegions = aws.Regions

// allInstanceTypes holds the relevant attributes of every known
// instance type.
// Note that while the EC2 root disk default is 8G, constraints on disk
// for amazon will simply cause the root disk to grow to match the constraint
var allInstanceTypes = []instances.InstanceType{
	{ // First generation.
		Name:     "m1.small",
		Arches:   both,
		CpuCores: 1,
		CpuPower: instances.CpuPower(100),
		Mem:      1740,
		VirtType: &paravirtual,
	}, {
		Name:     "m1.medium",
		Arches:   both,
		CpuCores: 1,
		CpuPower: instances.CpuPower(200),
		Mem:      3840,
		VirtType: &paravirtual,
	}, {
		Name:     "m1.large",
		Arches:   amd64,
		CpuCores: 2,
		CpuPower: instances.CpuPower(400),
		Mem:      7680,
		VirtType: &paravirtual,
	}, {
		Name:     "m1.xlarge",
		Arches:   amd64,
		CpuCores: 4,
		CpuPower: instances.CpuPower(800),
		Mem:      15360,
		VirtType: &paravirtual,
	},
	{ // Second generation.
		Name:     "m3.medium",
		Arches:   amd64,
		CpuCores: 1,
		CpuPower: instances.CpuPower(300),
		Mem:      3840,
		VirtType: &paravirtual,
	}, {
		Name:     "m3.large",
		Arches:   amd64,
		CpuCores: 2,
		CpuPower: instances.CpuPower(6500),
		Mem:      7680,
		VirtType: &paravirtual,
	}, {
		Name:     "m3.xlarge",
		Arches:   amd64,
		CpuCores: 4,
		CpuPower: instances.CpuPower(1300),
		Mem:      15360,
		VirtType: &paravirtual,
	}, {
		Name:     "m3.2xlarge",
		Arches:   amd64,
		CpuCores: 8,
		CpuPower: instances.CpuPower(2600),
		Mem:      30720,
		VirtType: &paravirtual,
	},
	{ // Micro.
		Name:     "t1.micro",
		Arches:   both,
		CpuCores: 1,
		CpuPower: instances.CpuPower(20),
		Mem:      613,
		VirtType: &paravirtual,
	},
	{ // High-Memory.
		Name:     "m2.xlarge",
		Arches:   amd64,
		CpuCores: 2,
		CpuPower: instances.CpuPower(650),
		Mem:      17408,
		VirtType: &paravirtual,
	}, {
		Name:     "m2.2xlarge",
		Arches:   amd64,
		CpuCores: 4,
		CpuPower: instances.CpuPower(1300),
		Mem:      34816,
		VirtType: &paravirtual,
	}, {
		Name:     "m2.4xlarge",
		Arches:   amd64,
		CpuCores: 8,
		CpuPower: instances.CpuPower(2600),
		Mem:      69632,
		VirtType: &paravirtual,
	},
	{ // High-CPU.
		Name:     "c1.medium",
		Arches:   both,
		CpuCores: 2,
		CpuPower: instances.CpuPower(500),
		Mem:      1740,
		VirtType: &paravirtual,
	}, {
		Name:     "c1.xlarge",
		Arches:   amd64,
		CpuCores: 8,
		CpuPower: instances.CpuPower(2000),
		Mem:      7168,
		VirtType: &paravirtual,
	},
	{ // Cluster compute.
		Name:     "cc1.4xlarge",
		Arches:   amd64,
		CpuCores: 8,
		CpuPower: instances.CpuPower(3350),
		Mem:      23552,
		VirtType: &hvm,
	}, {
		Name:     "cc2.8xlarge",
		Arches:   amd64,
		CpuCores: 16,
		CpuPower: instances.CpuPower(8800),
		Mem:      61952,
		VirtType: &hvm,
	},
	{ // High Memory cluster.
		Name:     "cr1.8xlarge",
		Arches:   amd64,
		CpuCores: 16,
		CpuPower: instances.CpuPower(8800),
		Mem:      249856,
		VirtType: &hvm,
	},
	{ // Cluster GPU.
		Name:     "cg1.4xlarge",
		Arches:   amd64,
		CpuCores: 8,
		CpuPower: instances.CpuPower(3350),
		Mem:      22528,
		VirtType: &hvm,
	},
	{ // High I/O.
		Name:     "hi1.4xlarge",
		Arches:   amd64,
		CpuCores: 16,
		CpuPower: instances.CpuPower(3500),
		Mem:      61952,
		VirtType: &paravirtual,
	},
	{ // High storage.
		Name:     "hs1.8xlarge",
		Arches:   amd64,
		CpuCores: 16,
		CpuPower: instances.CpuPower(3500),
		Mem:      119808,
		VirtType: &paravirtual,
	},
}

type instanceTypeCost map[string]uint64
type regionCosts map[string]instanceTypeCost

// allRegionCosts holds the cost in USDe-3/hour for each available instance
// type in each region.
var allRegionCosts = regionCosts{
	"ap-northeast-1": { // Tokyo.
		"m1.small":   88,
		"m1.medium":  175,
		"m1.large":   350,
		"m1.xlarge":  700,
		"m3.medium":  171,
		"m3.large":   342,
		"m3.xlarge":  684,
		"m3.2xlarge": 1368,
		"t1.micro":   27,
		"m2.xlarge":  505,
		"m2.2xlarge": 1010,
		"m2.4xlarge": 2020,
		"c1.medium":  185,
		"c1.xlarge":  740,
	},
	"ap-southeast-1": { // Singapore.
		"m1.small":   80,
		"m1.medium":  160,
		"m1.large":   320,
		"m1.xlarge":  640,
		"m3.medium":  158,
		"m3.large":   315,
		"m3.xlarge":  730,
		"m3.2xlarge": 1260,
		"t1.micro":   20,
		"m2.xlarge":  495,
		"m2.2xlarge": 990,
		"m2.4xlarge": 1980,
		"c1.medium":  183,
		"c1.xlarge":  730,
	},
	"ap-southeast-2": { // Sydney.
		"m1.small":   80,
		"m1.medium":  160,
		"m1.large":   320,
		"m1.xlarge":  640,
		"m3.medium":  158,
		"m3.large":   315,
		"m3.xlarge":  730,
		"m3.2xlarge": 1260,
		"t1.micro":   20,
		"m2.xlarge":  495,
		"m2.2xlarge": 990,
		"m2.4xlarge": 1980,
		"c1.medium":  183,
		"c1.xlarge":  730,
	},
	"eu-west-1": { // Ireland.
		"m1.small":    65,
		"m1.medium":   130,
		"m1.large":    260,
		"m1.xlarge":   520,
		"m3.medium":   124,
		"m3.large":    248,
		"m3.xlarge":   495,
		"m3.2xlarge":  990,
		"t1.micro":    20,
		"m2.xlarge":   460,
		"m2.2xlarge":  920,
		"m2.4xlarge":  1840,
		"c1.medium":   165,
		"c1.xlarge":   660,
		"cc2.8xlarge": 2700,
		"cg1.4xlarge": 2360,
		"hi1.4xlarge": 3410,
	},
	"sa-east-1": { // Sao Paulo.
		"m1.small":   80,
		"m1.medium":  160,
		"m1.large":   320,
		"m1.xlarge":  640,
		"t1.micro":   27,
		"m3.medium":  153,
		"m3.large":   306,
		"m3.xlarge":  612,
		"m3.2xlarge": 1224,
		"m2.xlarge":  540,
		"m2.2xlarge": 1080,
		"m2.4xlarge": 2160,
		"c1.medium":  200,
		"c1.xlarge":  800,
	},
	"us-east-1": { // Northern Virginia.
		"m1.small":    60,
		"m1.medium":   120,
		"m1.large":    240,
		"m1.xlarge":   480,
		"m3.medium":   113,
		"m3.large":    225,
		"m3.xlarge":   450,
		"m3.2xlarge":  900,
		"t1.micro":    20,
		"m2.xlarge":   410,
		"m2.2xlarge":  820,
		"m2.4xlarge":  1640,
		"c1.medium":   145,
		"c1.xlarge":   580,
		"cc1.4xlarge": 1300,
		"cc2.8xlarge": 2400,
		"cr1.8xlarge": 3500,
		"cg1.4xlarge": 2100,
		"hi1.4xlarge": 3100,
		"hs1.8xlarge": 4600,
	},
	"us-west-1": { // Northern California.
		"m1.small":   65,
		"m1.medium":  130,
		"m1.large":   260,
		"m1.xlarge":  520,
		"m3.medium":  124,
		"m3.large":   248,
		"m3.xlarge":  495,
		"m3.2xlarge": 990,
		"t1.micro":   25,
		"m2.xlarge":  460,
		"m2.2xlarge": 920,
		"m2.4xlarge": 1840,
		"c1.medium":  165,
		"c1.xlarge":  660,
	},
	"us-west-2": { // Oregon.
		"m1.small":    60,
		"m1.medium":   120,
		"m1.large":    240,
		"m1.xlarge":   480,
		"m3.medium":   113,
		"m3.large":    225,
		"m3.xlarge":   450,
		"m3.2xlarge":  900,
		"t1.micro":    20,
		"m2.xlarge":   410,
		"m2.2xlarge":  820,
		"m2.4xlarge":  1640,
		"c1.medium":   145,
		"c1.xlarge":   580,
		"cc2.8xlarge": 2400,
		"cr1.8xlarge": 3500,
		"hi1.4xlarge": 3100,
	},
}
