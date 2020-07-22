package describe

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/openshift/odo/pkg/catalog"
	"github.com/openshift/odo/pkg/devfile"
	"github.com/openshift/odo/pkg/devfile/parser"
	"github.com/openshift/odo/pkg/devfile/parser/data/common"
	"github.com/openshift/odo/pkg/log"
	"github.com/openshift/odo/pkg/machineoutput"
	"github.com/openshift/odo/pkg/odo/genericclioptions"
	"github.com/openshift/odo/pkg/odo/util/experimental"
	"github.com/openshift/odo/pkg/odo/util/pushtarget"
	"github.com/openshift/odo/pkg/util"
	pkgUtil "github.com/openshift/odo/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	"k8s.io/klog"
	ktemplates "k8s.io/kubectl/pkg/util/templates"
)

const componentRecommendedCommandName = "component"

var (
	componentExample = ktemplates.Examples(`  # Describe a component
    %[1]s nodejs`)

	componentLongDesc = ktemplates.LongDesc(`Describe a component type.
This describes the component and its associated starter projects.
`)
)

// DescribeComponentOptions encapsulates the options for the odo catalog describe component command
type DescribeComponentOptions struct {
	// name of the component to describe, from command arguments
	componentName string
	// if devfile components with name that matches arg[0]
	devfileComponents []catalog.DevfileComponentType
	// if componentName is a classic/odov1 component
	component string
	// generic context options common to all commands
	*genericclioptions.Context
}

// NewDescribeComponentOptions creates a new DescribeComponentOptions instance
func NewDescribeComponentOptions() *DescribeComponentOptions {
	return &DescribeComponentOptions{}
}

// Complete completes DescribeComponentOptions after they've been created
func (o *DescribeComponentOptions) Complete(name string, cmd *cobra.Command, args []string) (err error) {
	o.componentName = args[0]
	tasks := util.NewConcurrentTasks(2)

	if !pushtarget.IsPushTargetDocker() {
		o.Context = genericclioptions.NewContext(cmd, true)

		tasks.Add(util.ConcurrentTask{ToRun: func(errChannel chan error) {
			catalogList, err := catalog.ListComponents(o.Client)
			if err != nil {
				if experimental.IsExperimentalModeEnabled() {
					klog.V(4).Info("Please log in to an OpenShift cluster to list OpenShift/s2i components")
				} else {
					errChannel <- err
				}
			}
			for _, image := range catalogList.Items {
				if image.Name == o.componentName {
					o.component = image.Name
				}
			}
		}})
	}

	if experimental.IsExperimentalModeEnabled() {
		tasks.Add(util.ConcurrentTask{ToRun: func(errChannel chan error) {
			catalogDevfileList, err := catalog.ListDevfileComponents("")
			if catalogDevfileList.DevfileRegistries == nil {
				log.Warning("Please run 'odo registry add <registry name> <registry URL>' to add registry for listing devfile components\n")
			}
			if err != nil {
				errChannel <- err
			}
			o.GetDevfileComponentsByName(catalogDevfileList)
		}})
	}

	return tasks.Run()
}

// Validate validates the DescribeComponentOptions based on completed values
func (o *DescribeComponentOptions) Validate() (err error) {
	if len(o.devfileComponents) == 0 && o.component == "" {
		return errors.Wrapf(err, "No components with the name \"%s\" found", o.componentName)
	}

	return nil
}

// Run contains the logic for the command associated with DescribeComponentOptions
func (o *DescribeComponentOptions) Run() (err error) {
	if experimental.IsExperimentalModeEnabled() {
		w := tabwriter.NewWriter(os.Stdout, 5, 2, 3, ' ', tabwriter.TabIndent)
		if log.IsJSON() {
			if len(o.devfileComponents) > 0 {
				for _, devfileComponent := range o.devfileComponents {
					devObj, err := GetDevfile(devfileComponent)
					if err != nil {
						return err
					}

					machineoutput.OutputSuccess(devObj)
				}
			}
		} else {
			if len(o.devfileComponents) > 1 {
				log.Warningf("There are multiple components named \"%s\" in different multiple devfile registries.\n", o.componentName)
			}
			if len(o.devfileComponents) > 0 {
				fmt.Fprintln(w, "Devfile Component(s):")

				for _, devfileComponent := range o.devfileComponents {
					fmt.Fprintln(w, "\n* Registry: "+devfileComponent.Registry.Name)

					devObj, err := GetDevfile(devfileComponent)
					if err != nil {
						return err
					}

					projects := devObj.Data.GetProjects()
					// only print project info if there is at least one project in the devfile
					err = o.PrintDevfileProjects(w, projects, devObj)
					if err != nil {
						return err
					}
				}
			} else {
				fmt.Fprintln(w, "There are no Odo devfile components with the name \""+o.componentName+"\"")
			}
			if o.component != "" {
				fmt.Fprintln(w, "\nS2I Based Components:")
				fmt.Fprintln(w, "-"+o.component)
			}
			fmt.Fprintln(w)
		}
	}

	return nil
}

// NewCmdCatalogDescribeComponent implements the odo catalog describe component command
func NewCmdCatalogDescribeComponent(name, fullName string) *cobra.Command {
	o := NewDescribeComponentOptions()
	command := &cobra.Command{
		Use:         name,
		Short:       "Describe a component",
		Long:        componentLongDesc,
		Example:     fmt.Sprintf(componentExample, fullName),
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"machineoutput": "json"},
		Run: func(cmd *cobra.Command, args []string) {
			genericclioptions.GenericRun(o, cmd, args)
		},
	}

	return command
}

// GetDevfileComponentsByName gets all the devfiles that have the same name as the specified components
func (o *DescribeComponentOptions) GetDevfileComponentsByName(catalogDevfileList catalog.DevfileComponentTypeList) {
	for _, devfileComponent := range catalogDevfileList.Items {
		if devfileComponent.Name == o.componentName {
			o.devfileComponents = append(o.devfileComponents, devfileComponent)
		}
	}
}

// GetDevfile downloads the devfile in memory and return the devfile object
func GetDevfile(devfileComponent catalog.DevfileComponentType) (parser.DevfileObj, error) {
	var devObj parser.DevfileObj

	data, err := pkgUtil.DownloadFileInMemory(devfileComponent.Registry.URL + devfileComponent.Link)
	if err != nil {
		return devObj, errors.Wrapf(err, "Failed to download devfile.yaml for devfile component: %s", devfileComponent.Name)
	}
	devObj, err = devfile.ParseInMemoryAndValidate(data)
	if err != nil {
		return devObj, err
	}
	return devObj, nil
}

// PrintDevfileProjects prints all the starter projects in a devfile
// If no starter projects exists in the devfile, it prints the whole devfile
func (o *DescribeComponentOptions) PrintDevfileProjects(w *tabwriter.Writer, projects []common.DevfileProject, devObj parser.DevfileObj) error {
	if len(projects) > 0 {
		fmt.Fprintln(w, "\nStarter Projects:")
		for _, project := range projects {
			yamlData, err := yaml.Marshal(project)
			if err != nil {
				return errors.Wrapf(err, "Failed to marshal devfile object into yaml")
			}
			fmt.Printf("---\n%s", string(yamlData))
		}
	} else {
		fmt.Fprintln(w, "The Odo devfile component \""+o.componentName+"\" has no starter projects.")
		yamlData, err := yaml.Marshal(devObj)
		if err != nil {
			return errors.Wrapf(err, "Failed to marshal devfile object into yaml")
		}
		fmt.Printf("---\n%s", string(yamlData))
	}
	return nil
}
