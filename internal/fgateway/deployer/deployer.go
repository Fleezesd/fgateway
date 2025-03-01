package deployer

import (
	"context"
	"io/fs"
	"path/filepath"
	"slices"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/storage"
	"helm.sh/helm/v3/pkg/storage/driver"

	"github.com/fleezesd/fgateway/internal/version"
	"github.com/fleezesd/fgateway/manifests/helm"
	"github.com/fleezesd/fgateway/pkg/utils/helmutil"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	api "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/samber/lo"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
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
	ControlPlane            *ControlPlaneInfo
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

// GetGvksToWatch returns the list of GVKs that the deployer will watch for
func (d *Deployer) GetGvksToWatch(ctx context.Context) ([]schema.GroupVersionKind, error) {
	// Create a default empty gateway for rendering
	defaultGateway := &api.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "default",
		},
	}

	// Define default helm values
	helmValues := map[string]any{
		"gateway": map[string]any{
			"istio": map[string]any{
				"enabled": false,
			},
			"image": map[string]any{},
		},
	}

	// Render chart objects
	objects, err := d.renderChartToObjects(defaultGateway, helmValues)
	if err != nil {
		return nil, err
	}

	// Extract unique GroupVersionKinds
	uniqueGVKs := make([]schema.GroupVersionKind, 0)
	for _, obj := range objects {
		gvk := obj.GetObjectKind().GroupVersionKind()
		if !slices.Contains(uniqueGVKs, gvk) {
			uniqueGVKs = append(uniqueGVKs, gvk)
		}
	}

	logger := log.FromContext(ctx)
	logger.V(1).Info("watching GVKs", "GVKs", uniqueGVKs)

	return uniqueGVKs, nil
}

// renderChartToObjects renders the Helm chart to Kubernetes objects and sets their namespace
func (d *Deployer) renderChartToObjects(gw *api.Gateway, vals map[string]any) ([]client.Object, error) {
	// Render the chart using gateway name and namespace
	objects, err := d.Render(gw.Name, gw.Namespace, vals)
	if err != nil {
		return nil, errors.Errorf("failed to render chart: %w", err)
	}

	// Ensure all objects are in the gateway's namespace
	for _, obj := range objects {
		obj.SetNamespace(gw.Namespace)
	}

	return objects, nil
}

// Render relies on a `helm install` to render the Chart with the injected values
// It returns the list of Objects that are rendered, and an optional error if rendering failed,
// or converting the rendered manifests to objects failed.
// Render generates Kubernetes objects from a Helm chart with the given name, namespace and values
func (d *Deployer) Render(name, ns string, vals map[string]any) ([]client.Object, error) {
	// Setup in-memory Helm storage
	storage := setupHelmStorage(ns)

	// Configure and prepare Helm install action
	install := prepareInstallAction(storage, name, ns)

	// Render the Helm chart
	release, err := install.RunWithContext(context.Background(), d.chart, vals)
	if err != nil {
		return nil, formatRenderError(err, ns, name)
	}

	// Convert rendered manifest to Kubernetes objects
	objects, err := helmutil.ConvertYAMLToObjects(d.cli.Scheme(), []byte(release.Manifest))
	if err != nil {
		return nil, formatConversionError(err, ns, name)
	}

	return objects, nil
}

// setupHelmStorage creates and configures in-memory Helm storage
func setupHelmStorage(namespace string) *action.Configuration {
	mem := driver.NewMemory()
	mem.SetNamespace(namespace)
	return &action.Configuration{
		Releases: storage.Init(mem),
	}
}

// prepareInstallAction configures a Helm install action for rendering
func prepareInstallAction(cfg *action.Configuration, name, namespace string) *action.Install {
	install := action.NewInstall(cfg)
	install.Namespace = namespace
	install.ReleaseName = name
	install.ClientOnly = true
	return install
}

// formatRenderError creates a formatted error for Helm chart rendering failures
func formatRenderError(err error, namespace, name string) error {
	return errors.Errorf("failed to render helm chart for gateway %s.%s: %w", namespace, name, err)
}

// formatConversionError creates a formatted error for YAML conversion failures
func formatConversionError(err error, namespace, name string) error {
	return errors.Errorf("failed to convert helm manifest yaml to objects for gateway %s.%s: %w", namespace, name, err)
}
