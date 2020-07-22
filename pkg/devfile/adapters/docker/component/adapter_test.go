package component

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/mount"
	volumeTypes "github.com/docker/docker/api/types/volume"
	"github.com/golang/mock/gomock"
	adaptersCommon "github.com/openshift/odo/pkg/devfile/adapters/common"
	devfileParser "github.com/openshift/odo/pkg/devfile/parser"
	"github.com/openshift/odo/pkg/devfile/parser/data/common"
	versionsCommon "github.com/openshift/odo/pkg/devfile/parser/data/common"
	"github.com/openshift/odo/pkg/lclient"
	"github.com/openshift/odo/pkg/testingutil"
)

func TestPush(t *testing.T) {

	testComponentName := "test"
	fakeClient := lclient.FakeNew()
	fakeErrorClient := lclient.FakeErrorNew()

	command := "ls -la"
	component := "alias1"
	workDir := "/root"

	// create a temp dir for the file indexer
	directory, err := ioutil.TempDir("", "")
	if err != nil {
		t.Errorf("TestPush error: error creating temporary directory for the indexer: %v", err)
	}

	pushParams := adaptersCommon.PushParameters{
		Path:              directory,
		WatchFiles:        []string{},
		WatchDeletedFiles: []string{},
		IgnoredFiles:      []string{},
		ForceBuild:        false,
	}

	execCommands := []versionsCommon.Exec{
		{
			CommandLine: command,
			Component:   component,
			Group: &versionsCommon.Group{
				Kind: versionsCommon.RunCommandGroupType,
			},
			WorkingDir: workDir,
		},
	}
	validComponents := []versionsCommon.DevfileComponent{
		{
			Container: &versionsCommon.Container{
				Name: component,
			},
		},
	}

	tests := []struct {
		name          string
		components    []versionsCommon.DevfileComponent
		componentType versionsCommon.DevfileComponentType
		client        *lclient.Client
		wantErr       bool
	}{
		{
			name:          "Case 1: Invalid devfile",
			componentType: "",
			components:    []versionsCommon.DevfileComponent{},
			client:        fakeClient,
			wantErr:       true,
		},
		{
			name:          "Case 2: Valid devfile",
			components:    validComponents,
			componentType: versionsCommon.ContainerComponentType,
			client:        fakeClient,
			wantErr:       false,
		},
		{
			name:          "Case 3: Valid devfile, docker client error",
			components:    validComponents,
			componentType: versionsCommon.ContainerComponentType,
			client:        fakeErrorClient,
			wantErr:       true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			devObj := devfileParser.DevfileObj{
				Data: testingutil.TestDevfileData{
					Components:   tt.components,
					ExecCommands: execCommands,
				},
			}

			adapterCtx := adaptersCommon.AdapterContext{
				ComponentName: testComponentName,
				Devfile:       devObj,
			}

			componentAdapter := New(adapterCtx, *tt.client)
			// ToDo: Add more meaningful unit tests once Push actually does something with its parameters
			err := componentAdapter.Push(pushParams)

			// Checks for unexpected error cases
			if !tt.wantErr == (err != nil) {
				t.Errorf("component adapter create unexpected error %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	// Remove the temp dir created for the file indexer
	err = os.RemoveAll(directory)
	if err != nil {
		t.Errorf("TestPush error: error deleting the temp dir %s", directory)
	}

}

func TestDockerTest(t *testing.T) {

	testComponentName := "test"
	fakeClient := lclient.FakeNew()
	fakeErrorClient := lclient.FakeErrorNew()

	command := "ls -la"
	component := "alias1"
	workDir := "/root"
	id := "testCmd"

	// create a temp dir for the file indexer
	directory, err := ioutil.TempDir("", "")
	if err != nil {
		t.Errorf("TestPush error: error creating temporary directory for the indexer: %v", err)
	}

	validComponents := []versionsCommon.DevfileComponent{
		{
			Container: &versionsCommon.Container{
				Name: component,
			},
		},
	}

	tests := []struct {
		name          string
		components    []versionsCommon.DevfileComponent
		componentType versionsCommon.DevfileComponentType
		client        *lclient.Client
		execCommands  []versionsCommon.Exec
		wantErr       bool
	}{
		{
			name:         "Case 1: Invalid devfile",
			components:   validComponents,
			execCommands: []versionsCommon.Exec{},
			client:       fakeClient,
			wantErr:      true,
		},
		{
			name:       "Case 2: Valid devfile",
			components: validComponents,
			client:     fakeClient,
			execCommands: []versionsCommon.Exec{
				{
					Id:          id,
					CommandLine: command,
					Component:   component,
					Group: &versionsCommon.Group{
						Kind: versionsCommon.TestCommandGroupType,
					},
					WorkingDir: workDir,
				},
			},
			wantErr: false,
		},
		{
			name:       "Case 3: Valid devfile, docker client error",
			components: validComponents,
			client:     fakeErrorClient,
			wantErr:    true,
		},
		{
			name:       "Case 4: No valid containers",
			components: []versionsCommon.DevfileComponent{},
			execCommands: []versionsCommon.Exec{
				{
					Id:          id,
					CommandLine: command,
					Component:   component,
					Group: &versionsCommon.Group{
						Kind: versionsCommon.TestCommandGroupType,
					},
					WorkingDir: workDir,
				},
			},
			client:  fakeClient,
			wantErr: true,
		},
		{
			name:       "Case 5: Invalid command",
			components: []versionsCommon.DevfileComponent{},
			execCommands: []versionsCommon.Exec{
				{
					Id:          id,
					CommandLine: "",
					Component:   component,
					Group: &versionsCommon.Group{
						Kind: versionsCommon.TestCommandGroupType,
					},
					WorkingDir: workDir,
				},
			},
			client:  fakeClient,
			wantErr: true,
		},
		{
			name:       "Case 6: No valid command group",
			components: []versionsCommon.DevfileComponent{},
			execCommands: []versionsCommon.Exec{
				{
					Id:          id,
					CommandLine: command,
					Component:   component,
					Group: &versionsCommon.Group{
						Kind: versionsCommon.RunCommandGroupType,
					},
					WorkingDir: workDir,
				},
			},
			client:  fakeClient,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			devObj := devfileParser.DevfileObj{
				Data: testingutil.TestDevfileData{
					Components:   tt.components,
					ExecCommands: tt.execCommands,
				},
			}

			adapterCtx := adaptersCommon.AdapterContext{
				ComponentName: testComponentName,
				Devfile:       devObj,
			}

			componentAdapter := New(adapterCtx, *tt.client)
			err := componentAdapter.Test(id, false)

			// Checks for unexpected error cases
			if !tt.wantErr == (err != nil) {
				t.Errorf("component adapter create unexpected error %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	// Remove the temp dir created for the file indexer
	err = os.RemoveAll(directory)
	if err != nil {
		t.Errorf("TestTest error: error deleting the temp dir %s", directory)
	}

}

func TestDoesComponentExist(t *testing.T) {
	fakeClient := lclient.FakeNew()
	fakeErrorClient := lclient.FakeErrorNew()

	tests := []struct {
		name             string
		client           *lclient.Client
		components       []common.DevfileComponent
		componentName    string
		getComponentName string
		want             bool
		wantErr          bool
	}{
		{
			name:   "Case 1: Valid component name",
			client: fakeClient,
			components: []common.DevfileComponent{
				testingutil.GetFakeComponent("alias1"),
				testingutil.GetFakeComponent("alias2"),
			},
			componentName:    "golang",
			getComponentName: "golang",
			want:             true,
			wantErr:          false,
		},
		{
			name:   "Case 2: Non-existent component name",
			client: fakeClient,
			components: []common.DevfileComponent{
				testingutil.GetFakeComponent("alias1"),
			},
			componentName:    "test",
			getComponentName: "fake-component",
			want:             false,
			wantErr:          false,
		},
		{
			name:             "Case 3: Container and devfile component mismatch",
			componentName:    "test",
			getComponentName: "golang",
			client:           fakeClient,
			components:       []common.DevfileComponent{},
			want:             true,
			wantErr:          true,
		},
		{
			name:   "Case 4: Docker client error",
			client: fakeErrorClient,
			components: []common.DevfileComponent{
				testingutil.GetFakeComponent("alias1"),
			},
			componentName:    "test",
			getComponentName: "fake-component",
			want:             false,
			wantErr:          true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			devObj := devfileParser.DevfileObj{
				Data: testingutil.TestDevfileData{
					Components: tt.components,
				},
			}

			adapterCtx := adaptersCommon.AdapterContext{
				ComponentName: tt.componentName,
				Devfile:       devObj,
			}

			componentAdapter := New(adapterCtx, *tt.client)

			// Verify that a component with the specified name exists
			componentExists, err := componentAdapter.DoesComponentExist(tt.getComponentName)
			if !tt.wantErr && err != nil {
				t.Errorf("TestDoesComponentExist error, unexpected error - %v", err)
			} else if !tt.wantErr && componentExists != tt.want {
				t.Errorf("expected %v, actual %v", tt.want, componentExists)
			} else if tt.wantErr && tt.want != componentExists {
				t.Errorf("expected %v, wanted %v, err %v", componentExists, tt.want, err)
			}

		})
	}

}

func TestAdapterDelete(t *testing.T) {
	type args struct {
		labels map[string]string
	}
	tests := []struct {
		name              string
		args              args
		componentName     string
		componentExists   bool
		skipContainerList bool
		wantErr           bool
	}{
		{
			name: "Case 1: component exists and given labels are valid",
			args: args{labels: map[string]string{
				"component": "component",
			}},
			componentName:   "component",
			componentExists: true,
			wantErr:         false,
		},
		{
			name:              "Case 2: component exists and given labels are not valid",
			args:              args{labels: nil},
			componentName:     "component",
			componentExists:   true,
			wantErr:           true,
			skipContainerList: true,
		},
		{
			name: "Case 3: component doesn't exists",
			args: args{labels: map[string]string{
				"component": "component",
			}},
			componentName:   "component",
			componentExists: false,
			wantErr:         false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			containerID := "my-id"
			volumeID := "my-volume-name"

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			devObj := devfileParser.DevfileObj{
				Data: testingutil.TestDevfileData{
					Components: []versionsCommon.DevfileComponent{},
				},
			}

			adapterCtx := adaptersCommon.AdapterContext{
				ComponentName: tt.componentName,
				Devfile:       devObj,
			}

			if !tt.componentExists {
				adapterCtx.ComponentName = "doesNotExists"
			}

			fkclient, mockDockerClient := lclient.FakeNewMockClient(ctrl)

			a := Adapter{
				Client:         *fkclient,
				AdapterContext: adapterCtx,
			}

			labeledContainers := []types.Container{}

			if tt.componentExists {
				labeledContainers = []types.Container{
					{
						ID: containerID,
						Labels: map[string]string{
							"component": tt.componentName,
						},
						Mounts: []types.MountPoint{
							{
								Type: mount.TypeVolume,
								Name: volumeID,
							},
						},
					},
				}

			}

			if !tt.skipContainerList {
				mockDockerClient.EXPECT().ContainerList(gomock.Any(), gomock.Any()).Return(labeledContainers, nil)

				if tt.componentExists {
					mockDockerClient.EXPECT().VolumeList(gomock.Any(), gomock.Any()).Return(volumeTypes.VolumeListOKBody{
						Volumes: []*types.Volume{
							{
								Name: volumeID,
								Labels: map[string]string{
									"component": tt.componentName,
									"type":      "projects",
								},
							},
						},
					}, nil)

					mockDockerClient.EXPECT().ContainerRemove(gomock.Any(), gomock.Eq(containerID), gomock.Any()).Return(nil)

					mockDockerClient.EXPECT().VolumeRemove(gomock.Any(), gomock.Eq(volumeID), gomock.Eq(true)).Return(nil)

				}
			}

			if err := a.Delete(tt.args.labels); (err != nil) != tt.wantErr {
				t.Errorf("Delete() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAdapterDeleteVolumes(t *testing.T) {

	// Convenience func to create a mock ODO-style container with the given volume mounts
	containerWithMount := func(componentName string, mountPoints []types.MountPoint) types.Container {

		return types.Container{
			ID: componentName,
			Labels: map[string]string{
				"component": componentName,
			},
			Mounts: mountPoints,
		}
	}

	componentName := "my-component"
	anotherComponentName := "other-component"

	// The purpose of these tests is to verify the correctness of container deletion, such as:
	// - Only volumes that match the format of an ODO-managed volume (storage or source) are deleted
	// - Ensure that bind mounts are never deleted
	// - Ensure that other component's volumes are never deleted
	// - Ensure that volumes that have only the exact source/storage labels format are deleted

	tests := []struct {
		name           string
		containers     []types.Container
		volumes        []*types.Volume
		expectToDelete []string
	}{
		{
			name: "Case 1: Should delete both storage and source mount",
			containers: []types.Container{
				containerWithMount(componentName,
					[]types.MountPoint{
						{
							Name: "my-src-mount",
							Type: mount.TypeVolume,
						},
						{
							Name: "my-storage-mount",
							Type: mount.TypeVolume,
						},
					}),
			},
			volumes: []*types.Volume{
				{
					Name: "my-src-mount",
					Labels: map[string]string{
						"component": componentName,
						"type":      "projects",
					},
				},
				{
					Name: "my-storage-mount",
					Labels: map[string]string{
						"component":    componentName,
						"storage-name": "anyval",
					},
				},
			},
			expectToDelete: []string{
				"my-src-mount",
				"my-storage-mount",
			},
		},
		{
			name: "Case 2: Should delete storage mount alone",
			containers: []types.Container{
				containerWithMount(componentName,
					[]types.MountPoint{
						{
							Name: "my-storage-mount",
							Type: mount.TypeVolume,
						},
					}),
			},
			volumes: []*types.Volume{
				{
					Name: "my-storage-mount",
					Labels: map[string]string{
						"component":    componentName,
						"storage-name": "anyval",
					},
				},
			},
			expectToDelete: []string{
				"my-storage-mount",
			},
		},
		{
			name: "Case 3: Should not delete a bind mount even if it matches src volume labels",
			containers: []types.Container{
				containerWithMount(componentName,
					[]types.MountPoint{
						{
							Name: "my-src-mount",
							Type: mount.TypeBind,
						},
					}),
			},

			volumes: []*types.Volume{
				{
					Name: "my-src-mount",
					Labels: map[string]string{
						"component": componentName,
						"type":      "projects",
					},
				},
			},
			expectToDelete: []string{},
		},
		{
			name: "Case 4: Should not try to delete other component's volumes",
			containers: []types.Container{
				containerWithMount(componentName,
					[]types.MountPoint{
						{
							Name: "my-src-mount",
							Type: mount.TypeVolume,
						},
						{
							Name: "my-storage-mount",
							Type: mount.TypeVolume,
						},
					}),
				containerWithMount(anotherComponentName,
					[]types.MountPoint{
						{
							Name: "my-src-mount-other-component",
							Type: mount.TypeVolume,
						},
						{
							Name: "my-storage-mount-other-component",
							Type: mount.TypeVolume,
						},
					}),
			},
			volumes: []*types.Volume{
				{
					Name: "my-src-mount",
					Labels: map[string]string{
						"component": componentName,
						"type":      "projects",
					},
				},
				{
					Name: "my-storage-mount",
					Labels: map[string]string{
						"component":    componentName,
						"storage-name": "anyval",
					},
				},
				{
					Name: "my-src-mount-other-component",
					Labels: map[string]string{
						"component": anotherComponentName,
						"type":      "projects",
					},
				},
				{
					Name: "my-storage-mount-other-component",
					Labels: map[string]string{
						"component":    anotherComponentName,
						"storage-name": "anyval",
					},
				},
			},
			expectToDelete: []string{
				"my-src-mount",
				"my-storage-mount",
			},
		},
		{
			name: "Case 5: Should not try to delete a component's non-ODO volumes, even if the format is very close to ODO",
			containers: []types.Container{containerWithMount("my-component",
				[]types.MountPoint{
					{
						Name: "my-src-mount",
						Type: mount.TypeVolume,
					},
					{
						Name: "my-storage-mount",
						Type: mount.TypeVolume,
					},
					{
						Name: "another-volume-1",
						Type: mount.TypeVolume,
					},
					{
						Name: "another-volume-2",
						Type: mount.TypeVolume,
					},
				})},
			volumes: []*types.Volume{
				{
					Name: "my-src-mount",
					Labels: map[string]string{
						"component": componentName,
						"type":      "projects",
					},
				},
				{
					Name: "my-storage-mount",
					Labels: map[string]string{
						"component":    componentName,
						"storage-name": "anyval",
					},
				},
				{
					Name: "another-volume-1",
					Labels: map[string]string{
						"component": componentName,
						"type":      "projects-but-not-really",
					},
				},
				{
					Name: "another-volume-2",
					Labels: map[string]string{
						"component":                   componentName,
						"storage-name-but-not-really": "anyval",
					},
				},
			},
			expectToDelete: []string{
				"my-src-mount",
				"my-storage-mount",
			},
		},
		{
			name: "Case 6: Should not delete a volume that is mounted into another container",
			containers: []types.Container{

				containerWithMount("my-component",
					[]types.MountPoint{
						{
							Name: "my-storage-mount",
							Type: mount.TypeVolume,
						},
					}),

				containerWithMount("a-non-odo-container-for-example",
					[]types.MountPoint{
						{
							Name: "my-storage-mount",
							Type: mount.TypeVolume,
						},
					}),
			},
			volumes: []*types.Volume{
				{
					Name: "my-storage-mount",
					Labels: map[string]string{
						"component":    componentName,
						"storage-name": "anyval",
					},
				},
			},
			expectToDelete: []string{},
		},
		{
			name: "Case 7: Should delete both storage and supervisord mount",
			containers: []types.Container{
				containerWithMount(componentName,
					[]types.MountPoint{
						{
							Name: "my-supervisord-mount",
							Type: mount.TypeVolume,
						},
						{
							Name: "my-storage-mount",
							Type: mount.TypeVolume,
						},
					}),
			},
			volumes: []*types.Volume{
				{
					Name: "my-supervisord-mount",
					Labels: map[string]string{
						"component": componentName,
						"type":      "supervisord",
						"image":     "supervisordimage",
						"version":   "supervisordversion",
					},
				},
				{
					Name: "my-storage-mount",
					Labels: map[string]string{
						"component":    componentName,
						"storage-name": "anyval",
					},
				},
			},
			expectToDelete: []string{
				"my-supervisord-mount",
				"my-storage-mount",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			devObj := devfileParser.DevfileObj{
				Data: testingutil.TestDevfileData{
					Components: []versionsCommon.DevfileComponent{},
				},
			}

			adapterCtx := adaptersCommon.AdapterContext{
				ComponentName: componentName,
				Devfile:       devObj,
			}

			fkclient, mockDockerClient := lclient.FakeNewMockClient(ctrl)

			a := Adapter{
				Client:         *fkclient,
				AdapterContext: adapterCtx,
			}

			arg := map[string]string{
				"component": componentName,
			}

			mockDockerClient.EXPECT().ContainerList(gomock.Any(), gomock.Any()).Return(tt.containers, nil)

			mockDockerClient.EXPECT().ContainerRemove(gomock.Any(), componentName, gomock.Any()).Return(nil)

			mockDockerClient.EXPECT().VolumeList(gomock.Any(), gomock.Any()).Return(volumeTypes.VolumeListOKBody{
				Volumes: tt.volumes,
			}, nil)

			for _, deleteExpected := range tt.expectToDelete {
				mockDockerClient.EXPECT().VolumeRemove(gomock.Any(), deleteExpected, gomock.Any()).Return(nil)
			}

			err := a.Delete(arg)
			if err != nil {
				t.Errorf("Delete() unexpected error = %v", err)
			}

		})

	}

}
