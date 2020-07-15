package component

import (
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/mount"
	"github.com/pkg/errors"
	"k8s.io/klog"

	"github.com/openshift/odo/pkg/devfile/adapters/common"
	versionsCommon "github.com/openshift/odo/pkg/devfile/parser/data/common"

	"github.com/openshift/odo/pkg/devfile/adapters/docker/storage"
	"github.com/openshift/odo/pkg/devfile/adapters/docker/utils"
	"github.com/openshift/odo/pkg/lclient"
	"github.com/openshift/odo/pkg/log"
	"github.com/openshift/odo/pkg/machineoutput"
	"github.com/openshift/odo/pkg/sync"
)

// New instantiantes a component adapter
func New(adapterContext common.AdapterContext, client lclient.Client) Adapter {

	var loggingClient machineoutput.MachineEventLoggingClient

	if log.IsJSON() {
		loggingClient = machineoutput.NewConsoleMachineEventLoggingClient()
	} else {
		loggingClient = machineoutput.NewNoOpMachineEventLoggingClient()
	}

	return Adapter{
		Client:             client,
		AdapterContext:     adapterContext,
		machineEventLogger: loggingClient,
	}
}

// Adapter is a component adapter implementation for Kubernetes
type Adapter struct {
	Client lclient.Client
	common.AdapterContext

	componentAliasToVolumes   map[string][]common.DevfileVolume
	uniqueStorage             []common.Storage
	volumeNameToDockerVolName map[string]string
	devfileInitCmd            string
	devfileBuildCmd           string
	devfileRunCmd             string
	supervisordVolumeName     string
	projectVolumeName         string
	machineEventLogger        machineoutput.MachineEventLoggingClient
}

// Push updates the component if a matching component exists or creates one if it doesn't exist
func (a Adapter) Push(parameters common.PushParameters) (err error) {
	componentExists, err := utils.ComponentExists(a.Client, a.Devfile.Data, a.ComponentName)
	if err != nil {
		return errors.Wrapf(err, "unable to determine if component %s exists", a.ComponentName)
	}

	// Process the volumes defined in the devfile
	a.componentAliasToVolumes = common.GetVolumes(a.Devfile)
	a.uniqueStorage, a.volumeNameToDockerVolName, err = storage.ProcessVolumes(&a.Client, a.ComponentName, a.componentAliasToVolumes)
	if err != nil {
		return errors.Wrapf(err, "unable to process volumes for component %s", a.ComponentName)
	}

	a.devfileInitCmd = parameters.DevfileInitCmd
	a.devfileBuildCmd = parameters.DevfileBuildCmd
	a.devfileRunCmd = parameters.DevfileRunCmd

	// Validate the devfile build and run commands
	log.Info("\nValidation")
	s := log.Spinner("Validating the devfile")
	pushDevfileCommands, err := common.ValidateAndGetPushDevfileCommands(a.Devfile.Data, a.devfileInitCmd, a.devfileBuildCmd, a.devfileRunCmd)
	if err != nil {
		s.End(false)
		return errors.Wrap(err, "failed to validate devfile build and run commands")
	}
	s.End(true)

	a.supervisordVolumeName, err = a.createAndInitSupervisordVolumeIfReqd(componentExists)
	if err != nil {
		return errors.Wrapf(err, "unable to create supervisord volume for component %s", a.ComponentName)
	}

	a.projectVolumeName, err = a.createProjectVolumeIfReqd()
	if err != nil {
		return errors.Wrapf(err, "unable to determine the project source volume for component %s", a.ComponentName)
	}

	if componentExists {
		componentExists, err = a.updateComponent()
	} else {
		err = a.createComponent()
	}

	if err != nil {
		return errors.Wrap(err, "unable to create or update component")
	}

	containers, err := utils.GetComponentContainers(a.Client, a.ComponentName)
	if err != nil {
		return errors.Wrapf(err, "error while retrieving container for odo component %s", a.ComponentName)
	}

	// Find at least one container with the source volume mounted, error out if none can be found
	containerID, sourceMount, err := getFirstContainerWithSourceVolume(containers)
	if err != nil {
		return errors.Wrapf(err, "error while retrieving container for odo component %s with a mounted project volume", a.ComponentName)
	}

	log.Infof("\nSyncing to component %s", a.ComponentName)
	// Get a sync adapter. Check if project files have changed and sync accordingly
	syncAdapter := sync.New(a.AdapterContext, &a.Client)

	// podChanged is defaulted to false, since docker volume is always present even if container goes down
	compInfo := common.ComponentInfo{
		ContainerName: containerID,
		SourceMount:   sourceMount,
	}
	syncParams := common.SyncParameters{
		PushParams:      parameters,
		CompInfo:        compInfo,
		ComponentExists: componentExists,
	}
	execRequired, err := syncAdapter.SyncFiles(syncParams)
	if err != nil {
		return errors.Wrapf(err, "failed to sync to component with name %s", a.ComponentName)
	}

	if execRequired {
		log.Infof("\nExecuting devfile commands for component %s", a.ComponentName)
		err = a.execDevfile(pushDevfileCommands, componentExists, parameters.Show, containers)
		if err != nil {
			return errors.Wrapf(err, "failed to execute devfile commands for component %s", a.ComponentName)
		}
	}

	return nil
}

// DoesComponentExist returns true if a component with the specified name exists, false otherwise
func (a Adapter) DoesComponentExist(cmpName string) (bool, error) {
	componentExists, err := utils.ComponentExists(a.Client, a.Devfile.Data, cmpName)
	return componentExists, err
}

// getFirstContainerWithSourceVolume returns the first container that set mountSources: true
// Because the source volume is shared across all components that need it, we only need to sync once,
// so we only need to find one container. If no container was found, that means there's no
// container to sync to, so return an error
func getFirstContainerWithSourceVolume(containers []types.Container) (string, string, error) {
	for _, c := range containers {
		for _, mount := range c.Mounts {
			if strings.Contains(mount.Name, lclient.ProjectSourceVolumeName) {
				return c.ID, mount.Destination, nil
			}
		}
	}

	return "", "", fmt.Errorf("in order to sync files, odo requires at least one component in a devfile to set 'mountSources: true'")
}

// Delete attempts to delete the component with the specified labels, returning an error if it fails
func (a Adapter) Delete(labels map[string]string) error {

	componentName, exists := labels["component"]
	if !exists {
		return errors.New("unable to delete component without a component label")
	}

	spinner := log.Spinnerf("Deleting devfile component %s", componentName)
	defer spinner.End(false)

	containers, err := a.Client.GetContainerList()
	if err != nil {
		return errors.Wrap(err, "unable to retrieve container list for delete operation")
	}

	// A unique list of volumes NOT to delete, because they are still mapped into other containers.
	// map key is volume name.
	volumesNotToDelete := map[string]string{}

	// Go through the containers which are NOT part of this component, and make a list of all
	// their volumes so we don't delete them.
	for _, container := range containers {

		if container.Labels["component"] == componentName {
			continue
		}

		for _, mount := range container.Mounts {
			volumesNotToDelete[mount.Name] = mount.Name
		}
	}

	componentContainer := a.Client.GetContainersByComponent(componentName, containers)

	if len(componentContainer) == 0 {
		spinner.End(false)
		log.Warningf("Component %s does not exist", componentName)
		return nil
	}

	allVolumes, err := a.Client.GetVolumes()
	if err != nil {
		return errors.Wrapf(err, "unable to retrieve list of all Docker volumes")
	}

	// Look for this component's volumes that contain either a storage-name label or a type label
	var vols []types.Volume
	for _, vol := range allVolumes {

		if vol.Labels["component"] == componentName {

			if snVal := vol.Labels["storage-name"]; len(strings.TrimSpace(snVal)) > 0 {
				vols = append(vols, vol)
			} else if typeVal := vol.Labels["type"]; typeVal == utils.ProjectsVolume {
				vols = append(vols, vol)
			} else if typeVal := vol.Labels["type"]; typeVal == utils.SupervisordVolume {
				vols = append(vols, vol)
			}
		}
	}

	// A unique list of volumes to delete; map key is volume name.
	volumesToDelete := map[string]string{}

	for _, container := range componentContainer {

		klog.V(4).Infof("Deleting container %s for component %s", container.ID, componentName)
		err = a.Client.RemoveContainer(container.ID)
		if err != nil {
			return errors.Wrapf(err, "unable to remove container ID %s of component %s", container.ID, componentName)
		}

		// Generate a list of mounted volumes of the odo-managed container
		volumeNames := map[string]string{}
		for _, m := range container.Mounts {

			if m.Type != mount.TypeVolume {
				continue
			}
			volumeNames[m.Name] = m.Name
		}

		for _, vol := range vols {

			// Don't delete any volumes which are mapped into other containers
			if _, exists := volumesNotToDelete[vol.Name]; exists {
				klog.V(4).Infof("Skipping volume %s as it is mapped into a non-odo managed container", vol.Name)
				continue
			}

			// If the volume was found to be attached to the component's container, then add the volume
			// to the deletion list.
			if _, ok := volumeNames[vol.Name]; ok {
				klog.V(4).Infof("Adding volume %s to deletion list", vol.Name)
				volumesToDelete[vol.Name] = vol.Name
			} else {
				klog.V(4).Infof("Skipping volume %s as it was not attached to the component's container", vol.Name)
			}
		}
	}

	// Finally, delete the volumes we discovered during container deletion.
	for name := range volumesToDelete {
		klog.V(4).Infof("Deleting the volume %s for component %s", name, componentName)
		err := a.Client.RemoveVolume(name)
		if err != nil {
			return errors.Wrapf(err, "Unable to remove volume %s of component %s", name, componentName)
		}
	}

	spinner.End(true)
	log.Successf("Successfully deleted component")

	return nil

}

// Log returns log from component
func (a Adapter) Log(follow, debug bool) (io.ReadCloser, error) {

	exists, err := utils.ComponentExists(a.Client, a.Devfile.Data, a.ComponentName)

	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, errors.Errorf("the component %s doesn't exist on the cluster", a.ComponentName)
	}

	containers, err := utils.GetComponentContainers(a.Client, a.ComponentName)
	if err != nil {
		return nil, errors.Wrapf(err, "error while retrieving container for odo component %s", a.ComponentName)
	}

	var command versionsCommon.DevfileCommand
	if debug {
		command, err = common.GetDebugCommand(a.Devfile.Data, "")
		if err != nil {
			return nil, err
		}
		if reflect.DeepEqual(versionsCommon.DevfileCommand{}, command) {
			return nil, errors.Errorf("no debug command found in devfile, please run \"odo log\" for run command logs")
		}

	} else {
		command, err = common.GetRunCommand(a.Devfile.Data, "")
		if err != nil {
			return nil, err
		}

	}
	containerID := utils.GetContainerIDForAlias(containers, command.Exec.Component)

	return a.Client.GetContainerLogs(containerID, follow)
}
