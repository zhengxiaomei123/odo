package component

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/openshift/odo/pkg/envinfo"
	"github.com/openshift/odo/pkg/odo/util/experimental"
	"github.com/openshift/odo/pkg/odo/util/pushtarget"

	"github.com/openshift/odo/pkg/util"

	"github.com/spf13/cobra"
	"k8s.io/klog"

	"github.com/openshift/odo/pkg/component"
	"github.com/openshift/odo/pkg/config"
	"github.com/openshift/odo/pkg/log"
	appCmd "github.com/openshift/odo/pkg/odo/cli/application"
	projectCmd "github.com/openshift/odo/pkg/odo/cli/project"
	"github.com/openshift/odo/pkg/odo/cli/ui"
	"github.com/openshift/odo/pkg/odo/genericclioptions"
	odoutil "github.com/openshift/odo/pkg/odo/util"
	"github.com/openshift/odo/pkg/odo/util/completion"

	ktemplates "k8s.io/kubectl/pkg/util/templates"
)

// DeleteRecommendedCommandName is the recommended delete command name
const DeleteRecommendedCommandName = "delete"

var deleteExample = ktemplates.Examples(`  # Delete component named 'frontend'. 
%[1]s frontend
%[1]s frontend --all
  `)

// DeleteOptions is a container to attach complete, validate and run pattern
type DeleteOptions struct {
	componentForceDeleteFlag bool
	componentDeleteAllFlag   bool
	componentDeleteWaitFlag  bool
	componentContext         string
	isCmpExists              bool
	*ComponentOptions

	// devfile path
	devfilePath     string
	namespace       string
	EnvSpecificInfo *envinfo.EnvSpecificInfo
}

// NewDeleteOptions returns new instance of DeleteOptions
func NewDeleteOptions() *DeleteOptions {
	return &DeleteOptions{false, false, false, "", false, &ComponentOptions{}, "", "", nil}
}

// Complete completes log args
func (do *DeleteOptions) Complete(name string, cmd *cobra.Command, args []string) (err error) {
	do.devfilePath = filepath.Join(do.componentContext, DevfilePath)

	// if experimental mode is enabled and devfile is present
	if experimental.IsExperimentalModeEnabled() && util.CheckPathExists(do.devfilePath) {
		do.EnvSpecificInfo, err = envinfo.NewEnvSpecificInfo(do.componentContext)
		if err != nil {
			return err
		}

		do.Context = genericclioptions.NewDevfileContext(cmd)
		if !pushtarget.IsPushTargetDocker() {
			// The namespace was retrieved from the --project flag (or from the kube client if not set) and stored in kclient when initalizing the context
			do.namespace = do.KClient.Namespace
		}

		return nil
	}

	do.Context = genericclioptions.NewContext(cmd)
	err = do.ComponentOptions.Complete(name, cmd, args)

	if do.componentContext == "" {
		do.componentContext = LocalDirectoryDefaultLocation
	}
	return
}

// Validate validates the list parameters
func (do *DeleteOptions) Validate() (err error) {
	// if experimental mode is enabled and devfile is present
	if experimental.IsExperimentalModeEnabled() && util.CheckPathExists(do.devfilePath) {
		return nil
	}

	if do.Context.Project == "" || do.Application == "" {
		return odoutil.ThrowContextError()
	}
	do.isCmpExists, err = component.Exists(do.Client, do.componentName, do.Application)
	if err != nil {
		return err
	}
	if !do.isCmpExists {
		log.Errorf("Component %s does not exist on the cluster", do.ComponentOptions.componentName)
		// If request is to delete non existing component without all flag, exit with exit code 1
		if !do.componentDeleteAllFlag {
			os.Exit(1)
		}
	}
	return
}

// Run has the logic to perform the required actions as part of command
func (do *DeleteOptions) Run() (err error) {
	klog.V(4).Infof("component delete called")
	klog.V(4).Infof("args: %#v", do)

	if experimental.IsExperimentalModeEnabled() && util.CheckPathExists(do.devfilePath) {
		return do.DevFileRun()
	}

	if do.isCmpExists {
		err = printDeleteComponentInfo(do.Client, do.componentName, do.Context.Application, do.Context.Project)
		if err != nil {
			return err
		}

		if do.componentForceDeleteFlag || ui.Proceed(fmt.Sprintf("Are you sure you want to delete %v from %v?", do.componentName, do.Application)) {
			// Before actually deleting the component, first unlink it from any component(s) in the cluster it might be linked to
			// We do this in three steps:
			// 1. Get list of active components in the cluster
			// 2. Use this list to find the components to which our component is linked and generate secret names that are linked
			// 3. Unlink these secrets from the components
			compoList, err := component.List(do.Client, do.Context.Application, do.LocalConfigInfo)
			if err != nil {
				return err
			}

			parentComponent, err := component.GetComponent(do.Client, do.componentName, do.Context.Application, do.Context.Project)
			if err != nil {
				return err
			}

			componentSecrets := component.UnlinkComponents(parentComponent, compoList)

			for component, secret := range componentSecrets {
				spinner := log.Spinner("Unlinking components")
				for _, secretName := range secret {

					defer spinner.End(false)

					err = do.Client.UnlinkSecret(secretName, component, do.Context.Application)
					if err != nil {
						log.Errorf("Unlinking failed")
						return err
					}

					spinner.End(true)
					log.Successf(fmt.Sprintf("Unlinked component %q from component %q for secret %q", parentComponent.Name, component, secretName))
				}
			}
			err = component.Delete(do.Client, do.componentDeleteWaitFlag, do.componentName, do.Application)
			if err != nil {
				return err
			}
			log.Successf("Component %s from application %s has been deleted", do.componentName, do.Application)

		} else {
			return fmt.Errorf("Aborting deletion of component: %v", do.componentName)
		}
	}

	if do.componentDeleteAllFlag {
		if do.componentForceDeleteFlag || ui.Proceed(fmt.Sprintf("Are you sure you want to delete local config for %v?", do.componentName)) {
			cfg, err := config.NewLocalConfigInfo(do.componentContext)
			if err != nil {
				return err
			}
			if err = util.DeleteIndexFile(do.componentContext); err != nil {
				return err
			}

			// this checks if the config file exists or not
			if err = cfg.DeleteConfigFile(); err != nil {
				return err
			}

			if err = cfg.DeleteConfigDirIfEmpty(); err != nil {
				return err
			}

			log.Successf("Config for the Component %s has been deleted", do.componentName)
		} else {
			return fmt.Errorf("Aborting deletion of config for component: %s", do.componentName)
		}
	}

	return
}

// Run has the logic to perform the required actions as part of command for devfiles
func (do *DeleteOptions) DevFileRun() (err error) {
	// devfile delete
	if do.componentForceDeleteFlag || ui.Proceed(fmt.Sprintf("Are you sure you want to delete the devfile component: %s?", do.EnvSpecificInfo.GetName())) {
		err = do.DevfileComponentDelete()
		if err != nil {
			log.Errorf("error occurred while deleting component, cause: %v", err)
		}
	} else {
		log.Error("Aborting deletion of component")
	}

	if do.componentDeleteAllFlag {
		// Prompt and delete env folder
		if do.componentForceDeleteFlag || ui.Proceed(fmt.Sprintf("Are you sure you want to delete env folder?")) {
			if !do.EnvSpecificInfo.EnvInfoFileExists() {
				return fmt.Errorf("env folder doesn't exist for the component")
			}
			if err = util.DeleteIndexFile(filepath.Dir(do.devfilePath)); err != nil {
				return err
			}

			err = do.EnvSpecificInfo.DeleteEnvInfoFile()
			if err != nil {
				return err
			}
			err = do.EnvSpecificInfo.DeleteEnvDirIfEmpty()
			if err != nil {
				return err
			}

			cfg, err := config.NewLocalConfigInfo(do.componentContext)
			if err != nil {
				return err
			}
			if err = cfg.DeleteConfigDirIfEmpty(); err != nil {
				return err
			}

			log.Successf("Successfully deleted env file")
		} else {
			log.Error("Aborting deletion of env folder")
		}

		// Prompt and delete devfile.yaml
		if do.componentForceDeleteFlag || ui.Proceed(fmt.Sprintf("Are you sure you want to delete devfile.yaml?")) {
			if !util.CheckPathExists(DevfilePath) {
				return fmt.Errorf("devfile.yaml does not exist in the current directory")
			}

			err = util.DeletePath(DevfilePath)
			if err != nil {
				return err
			}

			log.Successf("Successfully deleted devfile.yaml file")
		} else {
			log.Error("Aborting deletion of devfile.yaml file")
		}
	}

	return nil
}

// NewCmdDelete implements the delete odo command
func NewCmdDelete(name, fullName string) *cobra.Command {

	do := NewDeleteOptions()

	var componentDeleteCmd = &cobra.Command{
		Use:         fmt.Sprintf("%s <component_name>", name),
		Short:       "Delete component",
		Long:        "Delete component.",
		Example:     fmt.Sprintf(deleteExample, fullName),
		Args:        cobra.MaximumNArgs(1),
		Annotations: map[string]string{"command": "component"},
		Run: func(cmd *cobra.Command, args []string) {
			genericclioptions.GenericRun(do, cmd, args)
		},
	}

	componentDeleteCmd.Flags().BoolVarP(&do.componentForceDeleteFlag, "force", "f", false, "Delete component without prompting")
	componentDeleteCmd.Flags().BoolVarP(&do.componentDeleteAllFlag, "all", "a", false, "Delete component and local config")
	componentDeleteCmd.Flags().BoolVarP(&do.componentDeleteWaitFlag, "wait", "w", false, "Wait for complete deletion of component and its dependent")

	componentDeleteCmd.SetUsageTemplate(odoutil.CmdUsageTemplate)
	completion.RegisterCommandHandler(componentDeleteCmd, completion.ComponentNameCompletionHandler)
	//Adding `--context` flag
	genericclioptions.AddContextFlag(componentDeleteCmd, &do.componentContext)

	//Adding `--project` flag
	projectCmd.AddProjectFlag(componentDeleteCmd)
	//Adding `--application` flag
	appCmd.AddApplicationFlag(componentDeleteCmd)

	return componentDeleteCmd
}
