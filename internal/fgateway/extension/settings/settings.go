package settings

import "github.com/kelseyhightower/envconfig"

type Settings struct {
	EnableIstioIntegration bool
	EnableAutoMTLS         bool
	StsClusterName         string
	StsUri                 string
}

func BuildSettings() (*Settings, error) {
	settings := &Settings{}
	if err := envconfig.Process(
		"FGW", settings,
	); err != nil {
		return nil, err
	}
	return settings, nil
}
