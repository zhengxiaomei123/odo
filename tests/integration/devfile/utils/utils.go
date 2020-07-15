package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/openshift/odo/tests/helper"

	. "github.com/onsi/gomega"
)

func useProjectIfAvailable(args []string, project string) []string {
	if project != "" {
		args = append(args, "--project", project)
	}

	return args
}

// ExecDefaultDevfileCommands executes the default devfile commands
func ExecDefaultDevfileCommands(projectDirPath, cmpName, namespace string) {
	args := []string{"create", "java-springboot", cmpName}
	args = useProjectIfAvailable(args, namespace)
	helper.CmdShouldPass("odo", args...)

	helper.CopyExample(filepath.Join("source", "devfiles", "springboot", "project"), projectDirPath)
	helper.CopyExampleDevFile(filepath.Join("source", "devfiles", "springboot", "devfile.yaml"), filepath.Join(projectDirPath, "devfile.yaml"))

	args = []string{"push"}
	args = useProjectIfAvailable(args, namespace)
	output := helper.CmdShouldPass("odo", args...)
	helper.MatchAllInOutput(output, []string{
		"Executing defaultbuild command \"/artifacts/bin/build-container-full.sh\"",
		"Executing defaultrun command \"/artifacts/bin/start-server.sh\"",
	})
}

// ExecWithMissingBuildCommand executes odo push with a missing build command
func ExecWithMissingBuildCommand(projectDirPath, cmpName, namespace string) {
	args := []string{"create", "nodejs", cmpName}
	args = useProjectIfAvailable(args, namespace)
	helper.CmdShouldPass("odo", args...)

	helper.CopyExample(filepath.Join("source", "devfiles", "nodejs", "project"), projectDirPath)
	helper.CopyExampleDevFile(filepath.Join("source", "devfiles", "nodejs", "devfile-without-devbuild.yaml"), filepath.Join(projectDirPath, "devfile.yaml"))

	args = []string{"push"}
	args = useProjectIfAvailable(args, namespace)
	output := helper.CmdShouldPass("odo", args...)
	Expect(output).NotTo(ContainSubstring("Executing devbuild command"))
	Expect(output).To(ContainSubstring("Executing devrun command \"npm install && npm start\""))
}

// ExecWithMissingRunCommand executes odo push with a missing run command
func ExecWithMissingRunCommand(projectDirPath, cmpName, namespace string) {
	args := []string{"create", "nodejs", cmpName}
	args = useProjectIfAvailable(args, namespace)
	helper.CmdShouldPass("odo", args...)

	helper.CopyExample(filepath.Join("source", "devfiles", "nodejs", "project"), projectDirPath)
	helper.CopyExampleDevFile(filepath.Join("source", "devfiles", "nodejs", "devfile.yaml"), filepath.Join(projectDirPath, "devfile.yaml"))

	// Remove the run commands
	helper.ReplaceString(filepath.Join(projectDirPath, "devfile.yaml"), "kind: run", "kind: debug")

	args = []string{"push"}
	args = useProjectIfAvailable(args, namespace)
	output := helper.CmdShouldFail("odo", args...)
	Expect(output).NotTo(ContainSubstring("Executing devrun command"))
	Expect(output).To(ContainSubstring("the command group of kind \"run\" is not found in the devfile"))
}

// ExecWithCustomCommand executes odo push with a custom command
func ExecWithCustomCommand(projectDirPath, cmpName, namespace string) {
	args := []string{"create", "nodejs", cmpName}
	args = useProjectIfAvailable(args, namespace)
	helper.CmdShouldPass("odo", args...)

	helper.CopyExample(filepath.Join("source", "devfiles", "nodejs", "project"), projectDirPath)
	helper.CopyExampleDevFile(filepath.Join("source", "devfiles", "nodejs", "devfile.yaml"), filepath.Join(projectDirPath, "devfile.yaml"))

	args = []string{"push", "--build-command", "build", "--run-command", "run"}
	args = useProjectIfAvailable(args, namespace)
	output := helper.CmdShouldPass("odo", args...)
	helper.MatchAllInOutput(output, []string{
		"Executing build command \"npm install\"",
		"Executing run command \"npm start\"",
	})
}

// ExecWithWrongCustomCommand executes odo push with a wrong custom command
func ExecWithWrongCustomCommand(projectDirPath, cmpName, namespace string) {
	garbageCommand := "buildgarbage"

	args := []string{"create", "nodejs", cmpName}
	args = useProjectIfAvailable(args, namespace)
	helper.CmdShouldPass("odo", args...)

	helper.CopyExample(filepath.Join("source", "devfiles", "nodejs", "project"), projectDirPath)
	helper.CopyExampleDevFile(filepath.Join("source", "devfiles", "nodejs", "devfile.yaml"), filepath.Join(projectDirPath, "devfile.yaml"))

	args = []string{"push", "--build-command", garbageCommand}
	args = useProjectIfAvailable(args, namespace)
	output := helper.CmdShouldFail("odo", args...)
	Expect(output).NotTo(ContainSubstring("Executing buildgarbage command"))
	Expect(output).To(ContainSubstring("the command \"%v\" is not found in the devfile", garbageCommand))
}

// ExecWithMultipleOrNoDefaults executes odo push with multiple or no default commands
func ExecWithMultipleOrNoDefaults(projectDirPath, cmpName, namespace string) {
	args := []string{"create", "nodejs", cmpName}
	args = useProjectIfAvailable(args, namespace)
	helper.CmdShouldPass("odo", args...)

	helper.CopyExample(filepath.Join("source", "devfiles", "nodejs", "project"), projectDirPath)
	helper.CopyExampleDevFile(filepath.Join("source", "devfiles", "nodejs", "devfile-with-multiple-defaults.yaml"), filepath.Join(projectDirPath, "devfile.yaml"))

	args = []string{"push"}
	args = useProjectIfAvailable(args, namespace)
	output := helper.CmdShouldFail("odo", args...)
	helper.MatchAllInOutput(output, []string{
		"there should be exactly one default command for command group build, currently there is more than one default command",
		"there should be exactly one default command for command group run, currently there is no default command",
	})
}

// ExecMultipleDefaultsWithFlags executes odo push with multiple default commands using flags
func ExecMultipleDefaultsWithFlags(projectDirPath, cmpName, namespace string) {
	args := []string{"create", "nodejs", cmpName}
	args = useProjectIfAvailable(args, namespace)
	helper.CmdShouldPass("odo", args...)

	helper.CopyExample(filepath.Join("source", "devfiles", "nodejs", "project"), projectDirPath)
	helper.CopyExampleDevFile(filepath.Join("source", "devfiles", "nodejs", "devfile-with-multiple-defaults.yaml"), filepath.Join(projectDirPath, "devfile.yaml"))

	args = []string{"push", "--build-command", "firstbuild", "--run-command", "secondrun"}
	args = useProjectIfAvailable(args, namespace)
	output := helper.CmdShouldPass("odo", args...)
	helper.MatchAllInOutput(output, []string{
		"Executing firstbuild command \"npm install\"",
		"Executing secondrun command \"npm start\"",
	})
}

// ExecCommandWithoutGroupUsingFlags executes odo push with no command group using flags
func ExecCommandWithoutGroupUsingFlags(projectDirPath, cmpName, namespace string) {
	args := []string{"create", "nodejs", cmpName}
	args = useProjectIfAvailable(args, namespace)
	helper.CmdShouldPass("odo", args...)

	helper.CopyExample(filepath.Join("source", "devfiles", "nodejs", "project"), projectDirPath)
	helper.CopyExampleDevFile(filepath.Join("source", "devfiles", "nodejs", "devfile-with-multiple-defaults.yaml"), filepath.Join(projectDirPath, "devfile.yaml"))

	args = []string{"push", "--build-command", "thirdbuild", "--run-command", "secondrun"}
	args = useProjectIfAvailable(args, namespace)
	output := helper.CmdShouldPass("odo", args...)
	helper.MatchAllInOutput(output, []string{
		"Executing thirdbuild command \"npm install\"",
		"Executing secondrun command \"npm start\"",
	})
}

// ExecWithInvalidCommandGroup executes odo push with an invalid command group
func ExecWithInvalidCommandGroup(projectDirPath, cmpName, namespace string) {
	args := []string{"create", "java-springboot", cmpName}
	args = useProjectIfAvailable(args, namespace)
	helper.CmdShouldPass("odo", args...)

	helper.CopyExample(filepath.Join("source", "devfiles", "springboot", "project"), projectDirPath)
	helper.CopyExampleDevFile(filepath.Join("source", "devfiles", "springboot", "devfile.yaml"), filepath.Join(projectDirPath, "devfile.yaml"))

	// Remove the run commands
	helper.ReplaceString(filepath.Join(projectDirPath, "devfile.yaml"), "kind: build", "kind: init")

	args = []string{"push"}
	args = useProjectIfAvailable(args, namespace)
	output := helper.CmdShouldFail("odo", args...)
	Expect(output).To(ContainSubstring("must be one of the following: \"build\", \"run\", \"test\", \"debug\""))
}

// ExecPushToTestFileChanges executes odo push with and without a file change
func ExecPushToTestFileChanges(projectDirPath, cmpName, namespace string) {
	args := []string{"create", "nodejs", cmpName}
	args = useProjectIfAvailable(args, namespace)
	helper.CmdShouldPass("odo", args...)

	helper.CopyExample(filepath.Join("source", "devfiles", "nodejs", "project"), projectDirPath)
	helper.CopyExampleDevFile(filepath.Join("source", "devfiles", "nodejs", "devfile.yaml"), filepath.Join(projectDirPath, "devfile.yaml"))

	args = []string{"push"}
	args = useProjectIfAvailable(args, namespace)
	helper.CmdShouldPass("odo", args...)

	output := helper.CmdShouldPass("odo", args...)
	Expect(output).To(ContainSubstring("No file changes detected, skipping build"))

	helper.ReplaceString(filepath.Join(projectDirPath, "server.js"), "Hello from Node.js", "UPDATED!")
	output = helper.CmdShouldPass("odo", args...)
	Expect(output).To(ContainSubstring("Syncing files to the component"))
}

// ExecPushWithForceFlag executes odo push with a force flag
func ExecPushWithForceFlag(projectDirPath, cmpName, namespace string) {
	args := []string{"create", "nodejs", cmpName}
	args = useProjectIfAvailable(args, namespace)
	helper.CmdShouldPass("odo", args...)

	helper.CopyExample(filepath.Join("source", "devfiles", "nodejs", "project"), projectDirPath)
	helper.CopyExampleDevFile(filepath.Join("source", "devfiles", "nodejs", "devfile.yaml"), filepath.Join(projectDirPath, "devfile.yaml"))

	args = []string{"push"}
	args = useProjectIfAvailable(args, namespace)
	helper.CmdShouldPass("odo", args...)

	// use the force build flag and push
	args = []string{"push", "-f"}
	args = useProjectIfAvailable(args, namespace)
	output := helper.CmdShouldPass("odo", args...)
	Expect(output).To(Not(ContainSubstring("No file changes detected, skipping build")))
}

// ExecPushWithNewFileAndDir executes odo push after creating a new file and dir
func ExecPushWithNewFileAndDir(projectDirPath, cmpName, namespace, newFilePath, newDirPath string) {
	args := []string{"create", "nodejs", cmpName}
	args = useProjectIfAvailable(args, namespace)
	helper.CmdShouldPass("odo", args...)

	helper.CopyExample(filepath.Join("source", "devfiles", "nodejs", "project"), projectDirPath)
	helper.CopyExampleDevFile(filepath.Join("source", "devfiles", "nodejs", "devfile.yaml"), filepath.Join(projectDirPath, "devfile.yaml"))

	// Create a new file that we plan on deleting later...
	if err := helper.CreateFileWithContent(newFilePath, "hello world"); err != nil {
		fmt.Printf("the foobar.txt file was not created, reason %v", err.Error())
	}

	// Create a new directory
	helper.MakeDir(newDirPath)

	// Push
	args = []string{"push"}
	args = useProjectIfAvailable(args, namespace)
	helper.CmdShouldPass("odo", args...)
}

// ExecWithRestartAttribute executes odo push with a command attribute restart
func ExecWithRestartAttribute(projectDirPath, cmpName, namespace string) {
	args := []string{"create", "nodejs", cmpName}
	args = useProjectIfAvailable(args, namespace)
	helper.CmdShouldPass("odo", args...)

	helper.CopyExample(filepath.Join("source", "devfiles", "nodejs", "project"), projectDirPath)
	helper.CopyExampleDevFile(filepath.Join("source", "devfiles", "nodejs", "devfile-with-restart.yaml"), filepath.Join(projectDirPath, "devfile.yaml"))

	args = []string{"push"}
	args = useProjectIfAvailable(args, namespace)
	output := helper.CmdShouldPass("odo", args...)
	Expect(output).To(ContainSubstring("Executing devrun command \"npm start\""))

	args = []string{"push", "-f"}
	args = useProjectIfAvailable(args, namespace)
	output = helper.CmdShouldPass("odo", args...)
	Expect(output).To(ContainSubstring("if not running"))

}

type OdoV1Watch struct {
	SrcType  string
	RouteURL string
	AppName  string
}

type OdoV2Watch struct {
	CmpName            string
	StringsToBeMatched []string
}

// OdoWatch creates files, dir in the context and watches for the changes to be pushed
// Specify OdoV1Watch for odo version 1, OdoV2Watch for odo version 2(devfile)
// platform is either kube or docker
func OdoWatch(odoV1Watch OdoV1Watch, odoV2Watch OdoV2Watch, project, context, flag string, runner interface{}, platform string) {

	isDevfileTest := false

	// if the odoV2Watch object is not empty, its a devfile test
	if !reflect.DeepEqual(odoV2Watch, OdoV2Watch{}) {
		isDevfileTest = true
	}

	startSimulationCh := make(chan bool)
	go func() {
		startMsg := <-startSimulationCh
		if startMsg {
			err := os.MkdirAll(filepath.Join(context, ".abc"), 0750)
			Expect(err).To(BeNil())

			err = os.MkdirAll(filepath.Join(context, "abcd"), 0750)
			Expect(err).To(BeNil())

			_, err = os.Create(filepath.Join(context, "a.txt"))
			Expect(err).To(BeNil())

			helper.DeleteDir(filepath.Join(context, "abcd"))

			if isDevfileTest {
				helper.ReplaceString(filepath.Join(context, "server.js"), "Hello", "Hello odo")
			} else {
				if odoV1Watch.SrcType == "openjdk" {
					helper.ReplaceString(filepath.Join(context, "src", "main", "java", "MessageProducer.java"), "Hello", "Hello odo")
				} else {
					helper.ReplaceString(filepath.Join(context, "server.js"), "Hello", "Hello odo")
				}
			}
		}
	}()

	if !isDevfileTest {
		flag = strings.TrimSpace(fmt.Sprintf("-v 4 %s", flag))
	}

	success, err := helper.WatchNonRetCmdStdOut(
		("odo watch " + flag + " --context " + context),
		time.Duration(5)*time.Minute,
		func(output string) bool {
			if isDevfileTest {
				stringsMatched := true

				for _, stringToBeMatched := range odoV2Watch.StringsToBeMatched {
					if !strings.Contains(output, stringToBeMatched) {
						stringsMatched = false
					}
				}

				if stringsMatched {
					// Verify delete from component pod
					err := validateContainerExecListDir(odoV1Watch, odoV2Watch, runner, platform, project, isDevfileTest)
					Expect(err).To(BeNil())
					return true
				}
			} else {
				curlURL := helper.CmdShouldPass("curl", odoV1Watch.RouteURL)
				if strings.Contains(curlURL, "Hello odo") {
					// Verify delete from component pod
					err := validateContainerExecListDir(odoV1Watch, odoV2Watch, runner, platform, project, isDevfileTest)
					Expect(err).To(BeNil())
					return true
				}
			}

			return false
		},
		startSimulationCh,
		func(output string) bool {
			return strings.Contains(output, "Waiting for something to change")
		})

	Expect(success).To(Equal(true))
	Expect(err).To(BeNil())

	if !isDevfileTest {
		// Verify memory limits to be same as configured
		getMemoryLimit := runner.(helper.OcRunner).MaxMemory(odoV1Watch.SrcType+"-app", odoV1Watch.AppName, project)
		Expect(getMemoryLimit).To(ContainSubstring("700Mi"))
		getMemoryRequest := runner.(helper.OcRunner).MinMemory(odoV1Watch.SrcType+"-app", odoV1Watch.AppName, project)
		Expect(getMemoryRequest).To(ContainSubstring("400Mi"))
	}
}

func validateContainerExecListDir(odoV1Watch OdoV1Watch, odoV2Watch OdoV2Watch, runner interface{}, platform, project string, isDevfileTest bool) error {
	var stdOut string

	switch platform {
	case "kube":
		if isDevfileTest {
			cliRunner := runner.(helper.CliRunner)
			podName := cliRunner.GetRunningPodNameByComponent(odoV2Watch.CmpName, project)
			stdOut = cliRunner.ExecListDir(podName, project, "/projects/nodejs-starter")
		} else {
			ocRunner := runner.(helper.OcRunner)
			podName := ocRunner.GetRunningPodNameOfComp(odoV1Watch.SrcType+"-app", project)
			envs := ocRunner.GetEnvs(odoV1Watch.SrcType+"-app", odoV1Watch.AppName, project)
			dir := envs["ODO_S2I_SRC_BIN_PATH"]
			stdOut = ocRunner.ExecListDir(podName, project, filepath.Join(dir, "src"))
		}
	case "docker":
		dockerRunner := runner.(helper.DockerRunner)
		containers := dockerRunner.GetRunningContainersByCompAlias(odoV2Watch.CmpName, "runtime")
		Expect(len(containers)).To(Equal(1))
		stdOut = dockerRunner.ExecContainer(containers[0], "ls -la /projects/nodejs-starter")
	default:
		return fmt.Errorf("Platform %s is not supported", platform)
	}

	helper.MatchAllInOutput(stdOut, []string{"a.txt", ".abc"})
	helper.DontMatchAllInOutput(stdOut, []string{"abcd"})

	return nil
}
