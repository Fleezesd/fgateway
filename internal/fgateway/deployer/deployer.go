package deployer

import (
	"io/fs"
	"path/filepath"

	"github.com/fleezesd/fgateway/internal/version"
	"github.com/fleezesd/fgateway/manifests/helm"
	"github.com/pkg/errors"

	"github.com/samber/lo"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	GetGatewayParametersError = errors.New("could not retrieve GatewayParameters")
	getGatewayParametersError = func(err error, gwpNamespace, gwpName, gwNamespace, gwName, resourceType string) error {
		wrapped := errors.Wrap(err, GetGatewayParametersError.Error())
		return errors.Wrapf(wrapped, "(%s.%s) for %s (%s.%s)",
			gwpNamespace, gwpName, resourceType, gwNamespace, gwName)
	}
	NilDeployerInputsErr = errors.New("nil inputs to NewDeployer")
)

type Deployer struct {
	chart *chart.Chart
	cli   client.Client

	inputs *Inputs
}

// Inputs is the set of options used to configure the gateway deployer deployment
type Inputs struct {
	ControllerName          string
	Dev                     bool
	IstioIntegrationEnabled bool
	ControlPlane            ControlPlaneInfo
	Aws                     *AwsInfo
}

type ControlPlaneInfo struct {
	XdsHost string
	XdsPort int32
}

type AwsInfo struct {
	EnableServiceAccountCredentials bool
	StsClusterName                  string
	StsUri                          string
}

// NewDeployer creates a new gateway deployer
func NewDeployer(cli client.Client, inputs *Inputs) (*Deployer, error) {
	if lo.IsNil(inputs) {
		return nil, NilDeployerInputsErr
	}

	helmChart, err := loadFs(helm.FGatewayHelmChart)
	if err != nil {
		return nil, err
	}
	// simulate what `helm package` in the Makefile does
	if version.Version != version.UndefinedVersion {
		helmChart.Metadata.AppVersion = version.Version
		helmChart.Metadata.Version = version.Version
	}
	return &Deployer{
		cli:    cli,
		inputs: inputs,
	}, nil
}

// loadFs use to load helm chart files
func loadFs(filesystem fs.FS) (*chart.Chart, error) {
	var bufferedFiles []*loader.BufferedFile
	entries, err := fs.ReadDir(filesystem, ".")
	if err != nil {
		return nil, err
	}
	if len(entries) != 1 {
		return nil, errors.Errorf("expected exactly one entry in the chart folder, got %v", entries)
	}

	root := entries[0].Name()

	// organize the helm chart file into the format required by helm
	err = fs.WalkDir(filesystem, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Skip directories - we only want files
		if d.IsDir() {
			return nil
		}
		data, readErr := fs.ReadFile(filesystem, path)
		if readErr != nil {
			return readErr
		}

		// Get the path relative to the root directory
		// This is needed because Helm expects relative paths
		relativePath, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}

		bufferedFile := &loader.BufferedFile{
			Name: relativePath,
			Data: data,
		}
		bufferedFiles = append(bufferedFiles, bufferedFile)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return loader.LoadFiles(bufferedFiles)
}
