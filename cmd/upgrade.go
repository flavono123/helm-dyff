package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/gonvenience/ytbx"
	"github.com/homeport/dyff/pkg/dyff"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/getter"
)

type upgradeCmdOptions struct {
	namespace string
	version   string
}

var upgradeCmdSettings upgradeCmdOptions
var valueOpts values.Options

var upgradeCmd = &cobra.Command{
	Use:   "upgrade [flags] <release> <chart>",
	Short: "Show a dyff on upgrading a release",
	Long: `
Show a dyff between the current release and the new chart.
`,
	Args:    cobra.ExactArgs(2),
	Aliases: []string{"u"}, // HACK: npm style??
	RunE:    runUpgrade,
}

func init() {
	upgradeCmd.Flags().StringVarP(&upgradeCmdSettings.namespace, "namespace", "n", "", "specify namespace where the release is installed, the currennt context's one would be used if not set")
	upgradeCmd.Flags().StringVarP(&upgradeCmdSettings.version, "version", "v", "", "specify the target chart version, the current release chart's one would be used if not set")
	// ref. https://github.dev/helm/helm/blob/ecc4adee692333629dbe6343fbcda58f8643b0ca/cmd/helm/flags.go#L45-L53
	addValueOptionsFlags(upgradeCmd.Flags(), &valueOpts)
}

func addValueOptionsFlags(f *pflag.FlagSet, v *values.Options) {
	f.StringSliceVarP(&v.ValueFiles, "values", "f", []string{}, "specify values in a YAML file or a URL (can specify multiple)")
	f.StringArrayVar(&v.Values, "set", []string{}, "set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.StringArrayVar(&v.StringValues, "set-string", []string{}, "set STRING values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.StringArrayVar(&v.FileValues, "set-file", []string{}, "set values from respective files specified via the command line (can specify multiple or separate values with commas: key1=path1,key2=path2)")
	f.StringArrayVar(&v.JSONValues, "set-json", []string{}, "set JSON values on the command line (can specify multiple or separate values with commas: key1=jsonval1,key2=jsonval2)")
	f.StringArrayVar(&v.LiteralValues, "set-literal", []string{}, "set a literal STRING value on the command line")
}

// runUpgrade do `dyff bw -b` with args as current release and desired to be installed,
// the concept of args have some scenarios:
// 1. upgrade the chart version with the same values
// dyff bw -ib <(helm get manifest <name> -n <namespace>) <(helm template -f <(helm get values -n <namespace> <name>) -n <namespace> <name> <chart> --version <chart-version>)
// 2. upgrade values with the same chart version
// dyff bw -ib <(helm get manifest <name> -n <namespace>) <(helm template -f <values-file> -n <namespace> <name> <chart>)
// 3. upgrade both chart version and values
// dyff bw -ib <(helm get manifest <name> -n <namespace>) <(helm template -f <values-file> -n <namespace> <name> <chart> --version <chart-version>)
func runUpgrade(_ *cobra.Command, args []string) error {
	releaseName, chartName := args[0], args[1]

	var namespace string

	if upgradeCmdSettings.namespace != "" {
		namespace = upgradeCmdSettings.namespace
	} else {
		namespace = getCurrentNamespace()
	}

	// 0. helm config
	envSettings := cli.New()
	cfg := new(action.Configuration)
	if err := cfg.Init(envSettings.RESTClientGetter(), namespace, "secrets", nil); err != nil {
		return err
	}

	// 1. <(helm get mainfest ...) get the current release mainfests by helm-template from the cluster
	helmGet := action.NewGet(cfg)
	release, err := helmGet.Run(releaseName)
	if err != nil {
		return err
	}

	// 2. get values
	// 2-1. from the values file
	// TODO: implement the values file

	// 2-2. from the current release
	helmGetValues := action.NewGetValues(cfg)
	currentReleaseValues, err := helmGetValues.Run(releaseName)
	if err != nil {
		return err
	}

	// 3. <(helm template ...) get the desired release mainfests by helm-template from the chart
	// helm package does not provide the API for helm-template, so we use helm-install with dry-run option instead
	helmInstall := action.NewInstall(cfg)
	helmInstall.DryRun = true
	helmInstall.ReleaseName = releaseName
	helmInstall.Namespace = namespace

	// HACK: override with the current release's one (this) or use latest, which is better?
	if upgradeCmdSettings.version != "" {
		helmInstall.ChartPathOptions.Version = upgradeCmdSettings.version
	} else {
		helmInstall.ChartPathOptions.Version = release.Chart.Metadata.Version
	}

	// chart
	chartPath, err := helmInstall.ChartPathOptions.LocateChart(chartName, envSettings)
	if err != nil {
		return err
	}

	chartRequested, err := loader.Load(chartPath)
	if err != nil {
		return err
	}

	// values
	p := getter.All(envSettings)
	vals, err := valueOpts.MergeValues(p)
	if err != nil {
		return err
	}

	var values map[string]interface{}

	if len(vals) == 0 {
		values = currentReleaseValues
	} else {
		values = vals
	}

	desiredRelease, err := helmInstall.Run(chartRequested, values)
	if err != nil {
		return err
	}

	fromFile, err := os.CreateTemp("", "helm-dyff-source-*")
	if err != nil {
		return err
	}
	defer os.Remove(fromFile.Name())
	if _, err := fromFile.Write([]byte(release.Manifest)); err != nil {
		return err
	}
	fromFile.Close()

	toFile, err := os.CreateTemp("", "helm-dyff-target-*")
	if err != nil {
		return err
	}
	defer os.Remove(toFile.Name())
	if _, err := toFile.Write([]byte(desiredRelease.Manifest)); err != nil {
		return err
	}
	toFile.Close()

	from, to, err := ytbx.LoadFiles(fromFile.Name(), toFile.Name())
	if err != nil {
		return err
	}
	report, err := dyff.CompareInputFiles(from, to,
		dyff.IgnoreOrderChanges(true), // -i
	)
	if err != nil {
		return err
	}

	// ref. https://github.dev/homeport/dyff/blob/e931f65dc633b24e1098900a3fb29fe4053957e9/internal/cmd/common.go#L208-L220
	rw := dyff.HumanReport{
		Report:     report,
		Indent:     2,
		OmitHeader: true, // -b
	}

	if err := rw.WriteReport(os.Stdout); err != nil {
		return err
	}

	return nil
}

func getCurrentNamespace() string {
	return os.Getenv("HELM_NAMESPACE")
}
