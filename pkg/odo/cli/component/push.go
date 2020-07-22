package component

import (
	"fmt"
	"path/filepath"

	"github.com/openshift/odo/pkg/devfile"
	"github.com/openshift/odo/pkg/envinfo"
	"github.com/openshift/odo/pkg/odo/util/pushtarget"
	ktemplates "k8s.io/kubectl/pkg/util/templates"

	"github.com/openshift/odo/pkg/component"
	"github.com/openshift/odo/pkg/devfile/parser"
	"github.com/openshift/odo/pkg/log"
	projectCmd "github.com/openshift/odo/pkg/odo/cli/project"
	"github.com/openshift/odo/pkg/odo/genericclioptions"
	"github.com/openshift/odo/pkg/odo/util/completion"
	"github.com/openshift/odo/pkg/odo/util/experimental"
	"github.com/openshift/odo/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	odoutil "github.com/openshift/odo/pkg/odo/util"
	"k8s.io/klog"
)

var pushCmdExample = (`  # Push source code to the current component
%[1]s

# Push data to the current component from the original source
%[1]s

# Push source code in ~/mycode to component called my-component
%[1]s my-component --context ~/mycode

# Push source code with custom devfile commands using --build-command and --run-command for experimental mode
%[1]s --build-command="mybuild" --run-command="myrun"
  `)

var pushCmdExampleExperimentalOnly = (`
# Output JSON events corresponding to devfile command execution and log text
%[1]s -o json
  `)

// PushRecommendedCommandName is the recommended push command name
const PushRecommendedCommandName = "push"

// PushOptions encapsulates options that push command uses
type PushOptions struct {
	*CommonPushOptions

	// devfile path
	DevfilePath string
	Devfile     parser.DevfileObj

	// devfile commands
	devfileInitCommand  string
	devfileBuildCommand string
	devfileRunCommand   string
	devfileDebugCommand string
	debugRun            bool

	namespace string
}

// NewPushOptions returns new instance of PushOptions
// with "default" values for certain values, for example, show is "false"
func NewPushOptions() *PushOptions {
	return &PushOptions{
		CommonPushOptions: NewCommonPushOptions(),
	}
}

// CompleteDevfilePath completes the devfile path from context
func (po *PushOptions) CompleteDevfilePath() {
	if len(po.DevfilePath) > 0 {
		po.DevfilePath = filepath.Join(po.componentContext, po.DevfilePath)
	} else {
		po.DevfilePath = filepath.Join(po.componentContext, "devfile.yaml")
	}
}

// Complete completes push args
func (po *PushOptions) Complete(name string, cmd *cobra.Command, args []string) (err error) {
	po.CompleteDevfilePath()

	// if experimental mode is enabled and devfile is present
	if experimental.IsExperimentalModeEnabled() && util.CheckPathExists(po.DevfilePath) {

		po.Devfile, err = devfile.ParseAndValidate(po.DevfilePath)
		if err != nil {
			return errors.Wrap(err, "unable to parse devfile")
		}

		// We retrieve the configuration information. If this does not exist, then BLANK is returned (important!).
		envFileInfo, err := envinfo.NewEnvSpecificInfo(po.componentContext)
		if err != nil {
			return errors.Wrap(err, "unable to retrieve configuration information")
		}

		// If the file does not exist, we should populate the environment file with the correct env.yaml information
		// such as name and namespace.
		if !envFileInfo.EnvInfoFileExists() {
			klog.V(4).Info("Environment file does not exist, creating the env.yaml file in order to use 'odo push'")

			// Since the environment file does not exist, we will retrieve a correct namespace from
			// either cmd commands or the current default kubernetes namespace
			namespace, err := retrieveCmdNamespace(cmd)
			if err != nil {
				return errors.Wrap(err, "unable to determine target namespace for devfile")
			}

			// Retrieve a default name
			// 1. Use args[0] if the user has supplied a name to be used
			// 2. If the user did not provide a name, use gatherName to retrieve a name from the devfile.Metadata
			// 3. Use the folder name that we are pushing from as a default name if none of the above exist

			var name string
			if len(args) == 1 {
				name = args[0]
			} else {
				name, err = gatherName(po.Devfile, po.DevfilePath)
				if err != nil {
					return errors.Wrap(err, "unable to gather a name to apply to the env.yaml file")
				}
			}

			// Create the environment file. This will actually *create* the env.yaml file in your context directory.
			err = envFileInfo.SetComponentSettings(envinfo.ComponentSettings{Name: name, Namespace: namespace})
			if err != nil {
				return errors.Wrap(err, "failed to create env.yaml for devfile component")
			}

		}

		po.EnvSpecificInfo = envFileInfo

		po.Context = genericclioptions.NewDevfileContext(cmd)

		// If the push target has been set to Docker, we will have to change the current namespace.
		// The namespace was retrieved from the --project flag (or from the kube client if not set) and stored in kclient when initalizing the context
		if !pushtarget.IsPushTargetDocker() {
			po.namespace = po.KClient.Namespace
		}

		return nil
	}

	// Set the correct context, which also sets the LocalConfigInfo
	po.Context = genericclioptions.NewContextCreatingAppIfNeeded(cmd)
	err = po.SetSourceInfo()
	if err != nil {
		return errors.Wrap(err, "unable to set source information")
	}

	// Apply ignore information
	err = genericclioptions.ApplyIgnore(&po.ignores, po.sourcePath)
	if err != nil {
		return errors.Wrap(err, "unable to apply ignore information")
	}

	// Get the project information and resolve it.
	prjName := po.LocalConfigInfo.GetProject()
	po.ResolveSrcAndConfigFlags()
	err = po.ResolveProject(prjName)
	if err != nil {
		return err
	}

	return nil
}

// Validate validates the push parameters
func (po *PushOptions) Validate() (err error) {

	// If the experimental flag is set and devfile is present, then we do *not* validate
	// TODO: We need to clean this section up a bit.. We should also validate Devfile here
	// too.
	if experimental.IsExperimentalModeEnabled() && util.CheckPathExists(po.DevfilePath) {
		return nil
	}

	log.Info("Validation")

	// First off, we check to see if the component exists. This is ran each time we do `odo push`
	s := log.Spinner("Checking component")
	defer s.End(false)

	po.doesComponentExist, err = component.Exists(po.Context.Client, po.LocalConfigInfo.GetName(), po.LocalConfigInfo.GetApplication())
	if err != nil {
		return errors.Wrapf(err, "failed to check if component of name %s exists in application %s", po.LocalConfigInfo.GetName(), po.LocalConfigInfo.GetApplication())
	}

	if err = component.ValidateComponentCreateRequest(po.Context.Client, po.LocalConfigInfo.GetComponentSettings(), po.componentContext); err != nil {
		s.End(false)
		log.Italic("\nRun 'odo catalog list components' for a list of supported component types")
		return fmt.Errorf("Invalid component type %s, %v", *po.LocalConfigInfo.GetComponentSettings().Type, errors.Cause(err))
	}

	if !po.doesComponentExist && po.pushSource && !po.pushConfig {
		return fmt.Errorf("Component %s does not exist and hence cannot push only source. Please use `odo push` without any flags or with both `--source` and `--config` flags", po.LocalConfigInfo.GetName())
	}

	s.End(true)
	return nil
}

// Run has the logic to perform the required actions as part of command
func (po *PushOptions) Run() (err error) {
	// If experimental mode is enabled, use devfile push
	if experimental.IsExperimentalModeEnabled() && util.CheckPathExists(po.DevfilePath) {
		// Return Devfile push
		return po.DevfilePush()
	} else {
		// Legacy odo push
		return po.Push()
	}
}

// NewCmdPush implements the push odo command
func NewCmdPush(name, fullName string) *cobra.Command {
	po := NewPushOptions()

	annotations := map[string]string{"command": "component"}

	pushCmdExampleText := pushCmdExample

	if experimental.IsExperimentalModeEnabled() {
		// The '-o json' option should only appear in help output when experimental mode is enabled.
		annotations["machineoutput"] = "json"

		// The '-o json' example should likewise only appear in experimental only.
		pushCmdExampleText += pushCmdExampleExperimentalOnly
	}

	var pushCmd = &cobra.Command{
		Use:         fmt.Sprintf("%s [component name]", name),
		Short:       "Push source code to a component",
		Long:        `Push source code to a component.`,
		Example:     fmt.Sprintf(ktemplates.Examples(pushCmdExampleText), fullName),
		Args:        cobra.MaximumNArgs(2),
		Annotations: annotations,
		Run: func(cmd *cobra.Command, args []string) {
			genericclioptions.GenericRun(po, cmd, args)
		},
	}

	genericclioptions.AddContextFlag(pushCmd, &po.componentContext)
	pushCmd.Flags().BoolVar(&po.show, "show-log", false, "If enabled, logs will be shown when built")
	pushCmd.Flags().StringSliceVar(&po.ignores, "ignore", []string{}, "Files or folders to be ignored via glob expressions.")
	pushCmd.Flags().BoolVar(&po.pushConfig, "config", false, "Use config flag to only apply config on to cluster")
	pushCmd.Flags().BoolVar(&po.pushSource, "source", false, "Use source flag to only push latest source on to cluster")
	pushCmd.Flags().BoolVarP(&po.forceBuild, "force-build", "f", false, "Use force-build flag to force building the component")

	// enable devfile flag if experimental mode is enabled
	if experimental.IsExperimentalModeEnabled() {
		pushCmd.Flags().StringVar(&po.namespace, "namespace", "", "Namespace to push the component to")
		pushCmd.Flags().StringVar(&po.devfileInitCommand, "init-command", "", "Devfile Init Command to execute")
		pushCmd.Flags().StringVar(&po.devfileBuildCommand, "build-command", "", "Devfile Build Command to execute")
		pushCmd.Flags().StringVar(&po.devfileRunCommand, "run-command", "", "Devfile Run Command to execute")
		pushCmd.Flags().BoolVar(&po.debugRun, "debug", false, "Runs the component in debug mode")
		pushCmd.Flags().StringVar(&po.devfileDebugCommand, "debug-command", "", "Devfile Debug Command to execute")
	}

	//Adding `--project` flag
	projectCmd.AddProjectFlag(pushCmd)

	pushCmd.SetUsageTemplate(odoutil.CmdUsageTemplate)
	completion.RegisterCommandHandler(pushCmd, completion.ComponentNameCompletionHandler)
	completion.RegisterCommandFlagHandler(pushCmd, "context", completion.FileCompletionHandler)

	return pushCmd
}
