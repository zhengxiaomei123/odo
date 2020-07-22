package devfile

import (
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/openshift/odo/tests/helper"
)

var _ = Describe("odo devfile test command tests", func() {
	var namespace, context, cmpName, currentWorkingDirectory, originalKubeconfig string
	var sourcePath = "/projects/nodejs-starter"

	// Using program commmand according to cliRunner in devfile
	cliRunner := helper.GetCliRunner()

	// This is run after every Spec (It)
	var _ = BeforeEach(func() {
		SetDefaultEventuallyTimeout(10 * time.Minute)
		context = helper.CreateNewContext()
		os.Setenv("GLOBALODOCONFIG", filepath.Join(context, "config.yaml"))
		helper.CmdShouldPass("odo", "preference", "set", "Experimental", "true")

		originalKubeconfig = os.Getenv("KUBECONFIG")
		helper.LocalKubeconfigSet(context)
		namespace = cliRunner.CreateRandNamespaceProject()
		currentWorkingDirectory = helper.Getwd()
		cmpName = helper.RandString(6)
		helper.Chdir(context)
	})

	// This is run after every Spec (It)
	var _ = AfterEach(func() {
		cliRunner.DeleteNamespaceProject(namespace)
		helper.Chdir(currentWorkingDirectory)
		err := os.Setenv("KUBECONFIG", originalKubeconfig)
		Expect(err).NotTo(HaveOccurred())
		helper.DeleteDir(context)
		os.Unsetenv("GLOBALODOCONFIG")
	})

	Context("Should show proper errors", func() {

		It("should show error if component is not pushed", func() {
			helper.CmdShouldPass("odo", "create", "nodejs", "--project", namespace, cmpName)

			helper.CopyExample(filepath.Join("source", "devfiles", "nodejs", "project"), context)
			helper.CopyExampleDevFile(filepath.Join("source", "devfiles", "nodejs", "devfile-with-testgroup.yaml"), filepath.Join(context, "devfile.yaml"))

			output := helper.CmdShouldFail("odo", "test", "--context", context)

			Expect(output).To(ContainSubstring("error occurred while getting the pod: no Pod was found for the selector"))
		})

		It("should show error for devfile v1", func() {
			helper.CmdShouldPass("odo", "create", "java-springboot", "--context", context, cmpName)

			helper.CopyExample(filepath.Join("source", "devfiles", "springboot", "project"), context)
			helper.CopyExampleDevFile(filepath.Join("source", "devfilesV1", "springboot", "devfile-init.yaml"), filepath.Join(context, "devfile.yaml"))

			output := helper.CmdShouldFail("odo", "test", "--context", context)

			Expect(output).To(ContainSubstring("'odo test' is not supported in devfile 1.0.0"))
		})

		It("should show error if no test group is defined", func() {
			helper.CmdShouldPass("odo", "create", "nodejs", "--context", context, cmpName)

			helper.CopyExample(filepath.Join("source", "devfiles", "nodejs", "project"), context)
			helper.CopyExampleDevFile(filepath.Join("source", "devfiles", "nodejs", "devfile.yaml"), filepath.Join(context, "devfile.yaml"))
			helper.CmdShouldPass("odo", "push", "--context", context)
			output := helper.CmdShouldFail("odo", "test", "--context", context)

			Expect(output).To(ContainSubstring("the command group of kind \"test\" is not found in the devfile"))
		})

		It("should show error if specify non-exist command", func() {
			helper.CmdShouldPass("odo", "create", "nodejs", "--context", context, cmpName)

			helper.CopyExample(filepath.Join("source", "devfiles", "nodejs", "project"), context)
			helper.CopyExampleDevFile(filepath.Join("source", "devfiles", "nodejs", "devfile-with-testgroup.yaml"), filepath.Join(context, "devfile.yaml"))
			helper.CmdShouldPass("odo", "push", "--context", context)
			output := helper.CmdShouldFail("odo", "test", "--test-command", "invalidcmd", "--context", context)

			Expect(output).To(ContainSubstring("not found in the devfile"))
		})

		It("should show error if command from another group", func() {
			helper.CmdShouldPass("odo", "create", "nodejs", "--context", context, cmpName)

			helper.CopyExample(filepath.Join("source", "devfiles", "nodejs", "project"), context)
			helper.CopyExampleDevFile(filepath.Join("source", "devfiles", "nodejs", "devfile-with-testgroup.yaml"), filepath.Join(context, "devfile.yaml"))
			helper.CmdShouldPass("odo", "push", "--context", context)
			output := helper.CmdShouldFail("odo", "test", "--test-command", "devrun", "--context", context)

			Expect(output).To(ContainSubstring("command devrun is of group run in devfile.yaml"))
		})

		It("should show error if devfile has no default test command", func() {
			helper.CmdShouldPass("odo", "create", "nodejs", "--context", context, cmpName)
			helper.CopyExample(filepath.Join("source", "devfiles", "nodejs", "project"), context)
			helper.CopyExampleDevFile(filepath.Join("source", "devfiles", "nodejs", "devfile-with-testgroup.yaml"), filepath.Join(context, "devfile.yaml"))
			helper.ReplaceString("devfile.yaml", "isDefault: true", "")
			helper.CmdShouldPass("odo", "push", "--context", context)
			output := helper.CmdShouldFail("odo", "test", "--context", context)
			Expect(output).To(ContainSubstring("there should be exactly one default command for command group test, currently there is no default command"))
		})

		It("should show error if devfile has multiple default test command", func() {
			helper.CmdShouldPass("odo", "create", "nodejs", "--context", context, cmpName)
			helper.CopyExample(filepath.Join("source", "devfiles", "nodejs", "project"), context)
			helper.CopyExampleDevFile(filepath.Join("source", "devfiles", "nodejs", "devfile-with-multiple-defaults.yaml"), filepath.Join(context, "devfile.yaml"))
			helper.CmdShouldPass("odo", "push", "--build-command", "firstbuild", "--run-command", "secondrun", "--context", context)
			output := helper.CmdShouldFail("odo", "test", "--context", context)
			Expect(output).To(ContainSubstring("there should be exactly one default command for command group test, currently there is more than one default command"))
		})
	})

	Context("Should run test command successfully", func() {

		It("Should run test command successfully with only one default specified", func() {
			helper.CmdShouldPass("odo", "create", "nodejs", "--context", context, cmpName)

			helper.CopyExample(filepath.Join("source", "devfiles", "nodejs", "project"), context)
			helper.CopyExampleDevFile(filepath.Join("source", "devfiles", "nodejs", "devfile-with-testgroup.yaml"), filepath.Join(context, "devfile.yaml"))
			helper.CmdShouldPass("odo", "push", "--context", context)

			output := helper.CmdShouldPass("odo", "test", "--context", context)
			helper.MatchAllInOutput(output, []string{"Executing test1 command", "mkdir test1"})

			podName := cliRunner.GetRunningPodNameByComponent(cmpName, namespace)
			output = cliRunner.ExecListDir(podName, namespace, sourcePath)
			Expect(output).To(ContainSubstring("test1"))
		})

		It("Should run test command successfully with test-command specified", func() {
			helper.CmdShouldPass("odo", "create", "nodejs", "--context", context, cmpName)

			helper.CopyExample(filepath.Join("source", "devfiles", "nodejs", "project"), context)
			helper.CopyExampleDevFile(filepath.Join("source", "devfiles", "nodejs", "devfile-with-testgroup.yaml"), filepath.Join(context, "devfile.yaml"))
			helper.CmdShouldPass("odo", "push", "--context", context)

			output := helper.CmdShouldPass("odo", "test", "--test-command", "test2", "--context", context)
			helper.MatchAllInOutput(output, []string{"Executing test2 command", "mkdir test2"})

			podName := cliRunner.GetRunningPodNameByComponent(cmpName, namespace)
			output = cliRunner.ExecListDir(podName, namespace, sourcePath)
			Expect(output).To(ContainSubstring("test2"))
		})

		It("should run test command successfully with test-command specified if devfile has no default test command", func() {
			helper.CmdShouldPass("odo", "create", "nodejs", "--context", context, cmpName)
			helper.CopyExample(filepath.Join("source", "devfiles", "nodejs", "project"), context)
			helper.CopyExampleDevFile(filepath.Join("source", "devfiles", "nodejs", "devfile-with-testgroup.yaml"), filepath.Join(context, "devfile.yaml"))
			helper.ReplaceString("devfile.yaml", "isDefault: true", "")
			helper.CmdShouldPass("odo", "push", "--context", context)
			output := helper.CmdShouldPass("odo", "test", "--test-command", "test2", "--context", context)
			helper.MatchAllInOutput(output, []string{"Executing test2 command", "mkdir test2"})

			podName := cliRunner.GetRunningPodNameByComponent(cmpName, namespace)
			output = cliRunner.ExecListDir(podName, namespace, sourcePath)
			Expect(output).To(ContainSubstring("test2"))
		})

		It("should run test command successfully with test-command specified if devfile has multiple default test command", func() {
			helper.CmdShouldPass("odo", "create", "nodejs", "--context", context, cmpName)
			helper.CopyExample(filepath.Join("source", "devfiles", "nodejs", "project"), context)
			helper.CopyExampleDevFile(filepath.Join("source", "devfiles", "nodejs", "devfile-with-multiple-defaults.yaml"), filepath.Join(context, "devfile.yaml"))
			helper.CmdShouldPass("odo", "push", "--build-command", "firstbuild", "--run-command", "secondrun", "--context", context)
			output := helper.CmdShouldPass("odo", "test", "--test-command", "test2", "--context", context)
			helper.MatchAllInOutput(output, []string{"Executing test2 command", "mkdir test2"})

			podName := cliRunner.GetRunningPodNameByComponent(cmpName, namespace)
			output = cliRunner.ExecListDir(podName, namespace, sourcePath)
			Expect(output).To(ContainSubstring("test2"))
		})
	})

})
