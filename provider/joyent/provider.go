// Copyright 2013 Joyent Inc.
// Licensed under the AGPLv3, see LICENCE file for details.

package joyent

import (
	"errors"
	"fmt"

	"github.com/joyent/gosign/auth"
	"github.com/juju/loggo"

	"github.com/juju/core/environs"
	"github.com/juju/core/environs/config"
	"github.com/juju/core/environs/imagemetadata"
	"github.com/juju/core/environs/simplestreams"
	envtools "github.com/juju/core/environs/tools"
)

var logger = loggo.GetLogger("juju.provider.joyent")

type joyentProvider struct{}

var providerInstance = joyentProvider{}
var _ environs.EnvironProvider = providerInstance

var _ simplestreams.HasRegion = (*joyentEnviron)(nil)
var _ imagemetadata.SupportsCustomSources = (*joyentEnviron)(nil)
var _ envtools.SupportsCustomSources = (*joyentEnviron)(nil)

func init() {
	environs.RegisterProvider("joyent", providerInstance)
}

var errNotImplemented = errors.New("not implemented in Joyent provider")

func (joyentProvider) Prepare(ctx environs.BootstrapContext, cfg *config.Config) (environs.Environ, error) {
	preparedCfg, err := prepareConfig(cfg)
	if err != nil {
		return nil, err
	}
	return providerInstance.Open(preparedCfg)
}

func credentials(cfg *environConfig) (*auth.Credentials, error) {
	authentication, err := auth.NewAuth(cfg.mantaUser(), cfg.privateKey(), cfg.algorithm())
	if err != nil {
		return nil, fmt.Errorf("cannot create credentials: %v", err)
	}
	return &auth.Credentials{
		UserAuthentication: authentication,
		MantaKeyId:         cfg.mantaKeyId(),
		MantaEndpoint:      auth.Endpoint{URL: cfg.mantaUrl()},
		SdcKeyId:           cfg.sdcKeyId(),
		SdcEndpoint:        auth.Endpoint{URL: cfg.sdcUrl()},
	}, nil
}

func (joyentProvider) Open(cfg *config.Config) (environs.Environ, error) {
	env, err := newEnviron(cfg)
	if err != nil {
		return nil, err
	}
	return env, nil
}

func (joyentProvider) Validate(cfg, old *config.Config) (valid *config.Config, err error) {
	newEcfg, err := validateConfig(cfg, old)
	if err != nil {
		return nil, fmt.Errorf("invalid Joyent provider config: %v", err)
	}
	return cfg.Apply(newEcfg.attrs)
}

func (joyentProvider) SecretAttrs(cfg *config.Config) (map[string]string, error) {
	// If you keep configSecretFields up to date, this method should Just Work.
	ecfg, err := validateConfig(cfg, nil)
	if err != nil {
		return nil, err
	}
	secretAttrs := map[string]string{}
	for _, field := range configSecretFields {
		if value, ok := ecfg.attrs[field]; ok {
			if stringValue, ok := value.(string); ok {
				secretAttrs[field] = stringValue
			} else {
				// All your secret attributes must be strings at the moment. Sorry.
				// It's an expedient and hopefully temporary measure that helps us
				// plug a security hole in the API.
				return nil, fmt.Errorf(
					"secret %q field must have a string value; got %v",
					field, value,
				)
			}
		}
	}
	return secretAttrs, nil
}

func (joyentProvider) BoilerplateConfig() string {
	return boilerplateConfig

}

func GetProviderInstance() environs.EnvironProvider {
	return providerInstance
}

// MetadataLookupParams returns parameters which are used to query image metadata to
// find matching image information.
func (p joyentProvider) MetadataLookupParams(region string) (*simplestreams.MetadataLookupParams, error) {
	if region == "" {
		return nil, fmt.Errorf("region must be specified")
	}
	return &simplestreams.MetadataLookupParams{
		Region:        region,
		Architectures: []string{"amd64", "armhf"},
	}, nil
}

func (p joyentProvider) newConfig(cfg *config.Config) (*environConfig, error) {
	valid, err := p.Validate(cfg, nil)
	if err != nil {
		return nil, err
	}
	return &environConfig{valid, valid.UnknownAttrs()}, nil
}
