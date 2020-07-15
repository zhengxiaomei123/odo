package component

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/openshift/odo/pkg/config"
	"github.com/openshift/odo/pkg/devfile/adapters"
	"github.com/openshift/odo/pkg/devfile/adapters/common"
	"github.com/openshift/odo/pkg/devfile/adapters/kubernetes"
	"github.com/openshift/odo/pkg/devfile/parser"
	"github.com/openshift/odo/pkg/occlient"
	appCmd "github.com/openshift/odo/pkg/odo/cli/application"
	projectCmd "github.com/openshift/odo/pkg/odo/cli/project"
	"github.com/openshift/odo/pkg/odo/util/experimental"
	"github.com/openshift/odo/pkg/odo/util/pushtarget"
	"github.com/pkg/errors"
	ktemplates "k8s.io/kubectl/pkg/util/templates"

	"github.com/openshift/odo/pkg/odo/genericclioptions"
	"k8s.io/klog"

	"github.com/openshift/odo/pkg/component"
	odoutil "github.com/openshift/odo/pkg/odo/util"
	"github.com/openshift/odo/pkg/util"
	"github.com/openshift/odo/pkg/watch"
	"github.com/spf13/cobra"
)

// WatchRecommendedCommandName is the recommended watch command name
const WatchRecommendedCommandName = "watch"

var watchLongDesc = ktemplates.LongDesc(`Watch for changes, update component on change. Watch doesn't support git components.`)
var watchExampleWithDevfile = ktemplates.Examples(`  # Watch for changes in directory for current component
%[1]s

# Watch source code changes with custom devfile commands using --build-command and --run-command for experimental mode
%[1]s --build-command="mybuild" --run-command="myrun"
  `)

var watchExample = ktemplates.Examples(`  # Watch for changes in directory for current component
%[1]s
  `)

// WatchOptions contains attributes of the watch command
type WatchOptions struct {
	ignores []string
	delay   int
	show    bool

	sourceType       config.SrcType
	sourcePath       string
	componentContext string
	client           *occlient.Client

	componentName  string
	devfilePath    string
	namespace      string
	devfileHandler common.ComponentAdapter

	// devfile commands
	devfileInitCommand  string
	devfileBuildCommand string
	devfileRunCommand   string

	*genericclioptions.Context
}

// NewWatchOptions returns new instance of WatchOptions
func NewWatchOptions() *WatchOptions {
	return &WatchOptions{}
}

// Complete completes watch args
func (wo *WatchOptions) Complete(name string, cmd *cobra.Command, args []string) (err error) {
	wo.devfilePath = filepath.Join(wo.componentContext, DevfilePath)

	// if experimental mode is enabled and devfile is present
	if experimental.IsExperimentalModeEnabled() && util.CheckPathExists(wo.devfilePath) {
		wo.Context = genericclioptions.NewDevfileContext(cmd)

		// Set the source path to either the context or current working directory (if context not set)
		wo.sourcePath, err = util.GetAbsPath(wo.componentContext)
		if err != nil {
			return errors.Wrap(err, "unable to get source path")
		}

		// Apply ignore information
		err = genericclioptions.ApplyIgnore(&wo.ignores, wo.sourcePath)
		if err != nil {
			return errors.Wrap(err, "unable to apply ignore information")
		}

		// Get the component name
		wo.componentName = wo.EnvSpecificInfo.GetName()
		if err != nil {
			return err
		}

		// Parse devfile and validate
		devObj, err := parser.ParseAndValidate(wo.devfilePath)
		if err != nil {
			return err
		}

		var platformContext interface{}
		if !pushtarget.IsPushTargetDocker() {
			// The namespace was retrieved from the --project flag (or from the kube client if not set) and stored in kclient when initalizing the context
			wo.namespace = wo.KClient.Namespace
			platformContext = kubernetes.KubernetesContext{
				Namespace: wo.namespace,
			}
		} else {
			platformContext = nil
		}
		wo.devfileHandler, err = adapters.NewComponentAdapter(wo.componentName, wo.componentContext, devObj, platformContext)

		return err
	}

	// Set the correct context
	wo.Context = genericclioptions.NewContextCreatingAppIfNeeded(cmd)

	wo.client = genericclioptions.Client(cmd)

	// Set the necessary values within WatchOptions
	conf := wo.Context.LocalConfigInfo
	wo.sourceType = conf.LocalConfig.GetSourceType()

	// Get SourceLocation here...
	wo.sourcePath, err = conf.GetOSSourcePath()
	if err != nil {
		return errors.Wrap(err, "unable to retrieve absolute path to source location")
	}

	// Apply ignore information
	err = genericclioptions.ApplyIgnore(&wo.ignores, wo.sourcePath)
	if err != nil {
		return errors.Wrap(err, "unable to apply ignore information")
	}

	return
}

// Validate validates the watch parameters
func (wo *WatchOptions) Validate() (err error) {

	// Delay interval cannot be -ve
	if wo.delay < 0 {
		return fmt.Errorf("Delay cannot be lesser than 0 and delay=0 means changes will be pushed as soon as they are detected which can cause performance issues")
	}
	// Print a debug message warning user if delay is set to 0
	if wo.delay == 0 {
		klog.V(4).Infof("delay=0 means changes will be pushed as soon as they are detected which can cause performance issues")
	}

	// if experimental mode is enabled and devfile is present, return. The rest of the validation is for non-devfile components
	if experimental.IsExperimentalModeEnabled() && util.CheckPathExists(wo.devfilePath) {
		exists, err := wo.devfileHandler.DoesComponentExist(wo.componentName)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("component does not exist. Please use `odo push` to create your component")
		}
		return nil
	}

	// Validate source of component is either local source or binary path until git watch is supported
	if wo.sourceType != "binary" && wo.sourceType != "local" {
		return fmt.Errorf("Watch is supported by binary and local components only and source type of component %s is %s",
			wo.LocalConfigInfo.GetName(),
			wo.sourceType)
	}

	// Validate component path existence and accessibility permissions for odo
	if _, err := os.Stat(wo.sourcePath); err != nil {
		return errors.Wrapf(err, "Cannot watch %s", wo.sourcePath)
	}

	cmpName := wo.LocalConfigInfo.GetName()
	appName := wo.LocalConfigInfo.GetApplication()
	if len(wo.Application) != 0 {
		appName = wo.Application
	}

	exists, err := component.Exists(wo.Client, cmpName, appName)
	if err != nil {
		return
	}
	if !exists {
		return fmt.Errorf("component does not exist. Please use `odo push` to create your component")
	}
	return
}

// Run has the logic to perform the required actions as part of command
func (wo *WatchOptions) Run() (err error) {
	// if experimental mode is enabled and devfile is present
	if experimental.IsExperimentalModeEnabled() && util.CheckPathExists(wo.devfilePath) {

		err = watch.DevfileWatchAndPush(
			os.Stdout,
			watch.WatchParameters{
				ComponentName:       wo.componentName,
				Path:                wo.sourcePath,
				FileIgnores:         util.GetAbsGlobExps(wo.sourcePath, wo.ignores),
				PushDiffDelay:       wo.delay,
				StartChan:           nil,
				ExtChan:             make(chan bool),
				DevfileWatchHandler: wo.devfileHandler.Push,
				Show:                wo.show,
				DevfileInitCmd:      strings.ToLower(wo.devfileInitCommand),
				DevfileBuildCmd:     strings.ToLower(wo.devfileBuildCommand),
				DevfileRunCmd:       strings.ToLower(wo.devfileRunCommand),
				EnvSpecificInfo:     wo.EnvSpecificInfo,
			},
		)
		if err != nil {
			return errors.Wrapf(err, "Error while trying to watch %s", wo.sourcePath)
		}
		return err
	}

	err = watch.WatchAndPush(
		wo.Context.Client,
		os.Stdout,
		watch.WatchParameters{
			ComponentName:   wo.LocalConfigInfo.GetName(),
			ApplicationName: wo.Context.Application,
			Path:            wo.sourcePath,
			FileIgnores:     util.GetAbsGlobExps(wo.sourcePath, wo.ignores),
			PushDiffDelay:   wo.delay,
			StartChan:       nil,
			ExtChan:         make(chan bool),
			WatchHandler:    component.PushLocal,
			Show:            wo.show,
		},
	)
	if err != nil {
		return errors.Wrapf(err, "Error while trying to watch %s", wo.sourcePath)
	}
	return
}

// NewCmdWatch implements the watch odo command
func NewCmdWatch(name, fullName string) *cobra.Command {
	wo := NewWatchOptions()

	example := fmt.Sprintf(watchExample, fullName)
	usage := name

	if experimental.IsExperimentalModeEnabled() {
		example = fmt.Sprintf(watchExampleWithDevfile, fullName)
	}

	var watchCmd = &cobra.Command{
		Use:         usage,
		Short:       "Watch for changes, update component on change. Watch doesn't support git components.",
		Long:        watchLongDesc,
		Example:     example,
		Args:        cobra.NoArgs,
		Annotations: map[string]string{"command": "component"},
		Run: func(cmd *cobra.Command, args []string) {
			genericclioptions.GenericRun(wo, cmd, args)
		},
	}

	watchCmd.Flags().BoolVar(&wo.show, "show-log", false, "If enabled, logs will be shown when built")
	watchCmd.Flags().StringSliceVar(&wo.ignores, "ignore", []string{}, "Files or folders to be ignored via glob expressions.")
	watchCmd.Flags().IntVar(&wo.delay, "delay", 1, "Time in seconds between a detection of code change and push.delay=0 means changes will be pushed as soon as they are detected which can cause performance issues")

	watchCmd.SetUsageTemplate(odoutil.CmdUsageTemplate)

	// enable devfile flag if experimental mode is enabled
	if experimental.IsExperimentalModeEnabled() {
		watchCmd.Flags().StringVar(&wo.devfileInitCommand, "init-command", "", "Devfile Init Command to execute")
		watchCmd.Flags().StringVar(&wo.devfileBuildCommand, "build-command", "", "Devfile Build Command to execute")
		watchCmd.Flags().StringVar(&wo.devfileRunCommand, "run-command", "", "Devfile Run Command to execute")
	}

	// Adding context flag
	genericclioptions.AddContextFlag(watchCmd, &wo.componentContext)

	//Adding `--application` flag
	appCmd.AddApplicationFlag(watchCmd)

	//Adding `--project` flag
	projectCmd.AddProjectFlag(watchCmd)

	return watchCmd
}
