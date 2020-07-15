package devfile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/openshift/odo/tests/helper"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("odo devfile url command tests", func() {
	var namespace, context, componentName, currentWorkingDirectory string

	// This is run after every Spec (It)
	var _ = BeforeEach(func() {
		SetDefaultEventuallyTimeout(10 * time.Minute)
		SetDefaultConsistentlyDuration(30 * time.Second)
		namespace = helper.CreateRandProject()
		context = helper.CreateNewContext()
		currentWorkingDirectory = helper.Getwd()
		componentName = helper.RandString(6)

		helper.Chdir(context)

		os.Setenv("GLOBALODOCONFIG", filepath.Join(context, "config.yaml"))

		// Devfile push requires experimental mode to be set
		helper.CmdShouldPass("odo", "preference", "set", "Experimental", "true")
	})

	// Clean up after the test
	// This is run after every Spec (It)
	var _ = AfterEach(func() {
		helper.DeleteProject(namespace)
		helper.Chdir(currentWorkingDirectory)
		helper.DeleteDir(context)
		os.Unsetenv("GLOBALODOCONFIG")
	})

	Context("Listing urls", func() {
		It("should list url after push", func() {
			url1 := helper.RandString(5)
			host := helper.RandString(5) + ".com"

			helper.CmdShouldPass("odo", "create", "nodejs", "--project", namespace, componentName)

			helper.CopyExample(filepath.Join("source", "devfiles", "nodejs", "project"), context)
			helper.CopyExampleDevFile(filepath.Join("source", "devfiles", "nodejs", "devfile.yaml"), filepath.Join(context, "devfile.yaml"))

			stdout := helper.CmdShouldFail("odo", "url", "list")
			helper.MatchAllInOutput(stdout, []string{
				"no URLs found",
				"Refer `odo url create -h` to add one",
			})

			stdout = helper.CmdShouldFail("odo", "url", "create", url1, "--port", "8080")
			Expect(stdout).To(ContainSubstring("is not exposed"))

			stdout = helper.CmdShouldFail("odo", "url", "create", url1, "--port", "3000", "--ingress")
			Expect(stdout).To(ContainSubstring("host must be provided"))

			helper.CmdShouldPass("odo", "url", "create", url1, "--port", "3000", "--host", host, "--ingress")
			helper.CmdShouldPass("odo", "push")
			helper.WaitForCmdOut("odo", []string{"url", "list"}, 1, false, func(output string) bool {
				if strings.Contains(output, url1) {
					Expect(output).Should(ContainSubstring(url1 + "." + host))
					return true
				}
				return false
			})
			helper.CmdShouldPass("odo", "url", "delete", url1, "-f")
			helper.CmdShouldPass("odo", "push", "--project", namespace)

			stdout = helper.CmdShouldFail("odo", "url", "list")
			Expect(stdout).To(ContainSubstring("no URLs found"))
		})

		It("should be able to list ingress url in machine readable json format", func() {
			url1 := helper.RandString(5)
			host := helper.RandString(5) + ".com"

			helper.CmdShouldPass("odo", "create", "nodejs", "--project", namespace, componentName)

			helper.CopyExample(filepath.Join("source", "devfiles", "nodejs", "project"), context)
			helper.CopyExampleDevFile(filepath.Join("source", "devfiles", "nodejs", "devfile.yaml"), filepath.Join(context, "devfile.yaml"))

			helper.CmdShouldPass("odo", "url", "create", url1, "--port", "3000", "--host", host, "--ingress")
			helper.CmdShouldPass("odo", "push", "--project", namespace)

			// odo url list -o json
			helper.WaitForCmdOut("odo", []string{"url", "list", "-o", "json"}, 1, true, func(output string) bool {
				desiredURLListJSON := fmt.Sprintf(`{"kind":"List","apiVersion":"odo.dev/v1alpha1","metadata":{},"items":[{"kind":"url","apiVersion":"odo.dev/v1alpha1","metadata":{"name":"%s","creationTimestamp":null},"spec":{"host":"%s","port":3000,"secure": false,"kind":"ingress"},"status":{"state":"Pushed"}}]}`, url1, url1+"."+host)
				if strings.Contains(output, url1) {
					Expect(desiredURLListJSON).Should(MatchJSON(output))
					return true
				}
				return false
			})
		})

		It("should list ingress url with appropriate state", func() {
			url1 := helper.RandString(5)
			url2 := helper.RandString(5)
			host := helper.RandString(5) + ".com"

			helper.CmdShouldPass("odo", "create", "nodejs", "--project", namespace, componentName)

			helper.CopyExample(filepath.Join("source", "devfiles", "nodejs", "project"), context)
			helper.CopyExampleDevFile(filepath.Join("source", "devfiles", "nodejs", "devfile.yaml"), filepath.Join(context, "devfile.yaml"))

			helper.CmdShouldPass("odo", "url", "create", url1, "--port", "3000", "--host", host, "--secure", "--ingress")
			helper.CmdShouldPass("odo", "push")
			helper.CmdShouldPass("odo", "url", "create", url2, "--port", "3000", "--host", host, "--ingress")
			stdout := helper.CmdShouldPass("odo", "url", "list")
			helper.MatchAllInOutput(stdout, []string{url1, "Pushed", "true", "ingress"})
			helper.MatchAllInOutput(stdout, []string{url2, "Not Pushed", "false", "ingress"})

			helper.CmdShouldPass("odo", "url", "delete", url1, "-f")
			stdout = helper.CmdShouldPass("odo", "url", "list")
			helper.MatchAllInOutput(stdout, []string{url1, "Locally Deleted", "true", "ingress"})
			helper.MatchAllInOutput(stdout, []string{url2, "Not Pushed", "false", "ingress"})
		})
	})

	Context("Creating urls", func() {
		It("should create a secure URL", func() {
			url1 := helper.RandString(5)
			host := helper.RandString(5) + ".com"

			helper.CmdShouldPass("odo", "create", "nodejs", "--project", namespace, componentName)

			helper.CopyExample(filepath.Join("source", "devfiles", "nodejs", "project"), context)
			helper.CopyExampleDevFile(filepath.Join("source", "devfiles", "nodejs", "devfile.yaml"), filepath.Join(context, "devfile.yaml"))

			helper.CmdShouldPass("odo", "url", "create", url1, "--port", "3000", "--host", host, "--secure", "--ingress")

			stdout := helper.CmdShouldPass("odo", "push", "--project", namespace)
			helper.MatchAllInOutput(stdout, []string{"https:", url1 + "." + host})
			stdout = helper.CmdShouldPass("odo", "url", "list")
			helper.MatchAllInOutput(stdout, []string{"https:", url1 + "." + host, "true"})
			helper.CmdShouldPass("odo", "url", "delete", url1, "-f")
			helper.CmdShouldPass("odo", "push", "--project", namespace)

			stdout = helper.CmdShouldFail("odo", "url", "list")
			Expect(stdout).To(ContainSubstring("no URLs found"))
		})

		It("create and delete with now flag should pass", func() {
			var stdout string
			url1 := helper.RandString(5)
			host := helper.RandString(5) + ".com"

			helper.CmdShouldPass("odo", "create", "nodejs", "--project", namespace, componentName)

			helper.CopyExample(filepath.Join("source", "devfiles", "nodejs", "project"), context)
			helper.CopyExampleDevFile(filepath.Join("source", "devfiles", "nodejs", "devfile.yaml"), filepath.Join(context, "devfile.yaml"))

			stdout = helper.CmdShouldPass("odo", "url", "create", url1, "--port", "3000", "--host", host, "--now", "--ingress")
			helper.MatchAllInOutput(stdout, []string{"URL " + url1 + " created for component", "http:", url1 + "." + host})
			stdout = helper.CmdShouldPass("odo", "url", "delete", url1, "--now", "-f")
			helper.MatchAllInOutput(stdout, []string{"URL " + url1 + " successfully deleted", "Applying URL changes"})
		})

		It("should create a url for a unsupported devfile component", func() {
			url1 := helper.RandString(5)

			helper.CopyExample(filepath.Join("source", "python"), context)
			helper.Chdir(context)

			helper.CmdShouldPass("odo", "create", "python", "--project", namespace, componentName)

			helper.CmdShouldPass("odo", "url", "create", url1)

			helper.CmdShouldPass("odo", "push", "--namespace", namespace)

			output := helper.CmdShouldPass("oc", "get", "routes", "--namespace", namespace)
			Expect(output).Should(ContainSubstring(url1))
		})

		It("should be able to push again twice after creating and deleting a url", func() {
			var stdOut string
			url1 := helper.RandString(5)
			host := helper.RandString(5) + ".com"

			helper.CmdShouldPass("odo", "create", "nodejs", "--project", namespace, componentName)

			helper.CopyExample(filepath.Join("source", "devfiles", "nodejs", "project"), context)
			helper.CopyExampleDevFile(filepath.Join("source", "devfiles", "nodejs", "devfile.yaml"), filepath.Join(context, "devfile.yaml"))

			helper.CmdShouldPass("odo", "url", "create", url1, "--port", "3000", "--host", host, "--ingress")

			helper.CmdShouldPass("odo", "push", "--project", namespace)
			stdOut = helper.CmdShouldPass("odo", "push", "--project", namespace)
			helper.DontMatchAllInOutput(stdOut, []string{"successfully deleted", "created"})
			Expect(stdOut).To(ContainSubstring("URLs are synced with the cluster, no changes are required"))

			helper.CmdShouldPass("odo", "url", "delete", url1, "-f")

			helper.CmdShouldPass("odo", "push", "--project", namespace)
			stdOut = helper.CmdShouldPass("odo", "push", "--project", namespace)
			helper.DontMatchAllInOutput(stdOut, []string{"successfully deleted", "created"})
			Expect(stdOut).To(ContainSubstring("URLs are synced with the cluster, no changes are required"))
		})
	})

	Context("Describing urls", func() {
		It("should describe appropriate Ingress URLs", func() {
			url1 := helper.RandString(5)
			host := helper.RandString(5) + ".com"

			helper.CmdShouldPass("odo", "create", "nodejs", "--project", namespace, componentName)

			helper.CopyExample(filepath.Join("source", "devfiles", "nodejs", "project"), context)
			helper.CopyExampleDevFile(filepath.Join("source", "devfiles", "nodejs", "devfile.yaml"), filepath.Join(context, "devfile.yaml"))

			helper.CmdShouldPass("odo", "url", "create", url1, "--port", "3000", "--host", host, "--ingress")

			stdout := helper.CmdShouldPass("odo", "url", "describe", url1)
			helper.MatchAllInOutput(stdout, []string{url1 + "." + host, "Not Pushed", "false", "ingress", "odo push"})

			helper.CmdShouldPass("odo", "push", "--project", namespace)
			stdout = helper.CmdShouldPass("odo", "url", "describe", url1)
			helper.MatchAllInOutput(stdout, []string{url1 + "." + host, "Pushed", "false", "ingress"})
			helper.CmdShouldPass("odo", "url", "delete", url1, "-f")
			stdout = helper.CmdShouldPass("odo", "url", "describe", url1)
			helper.MatchAllInOutput(stdout, []string{url1 + "." + host, "Locally Deleted", "false", "ingress"})

			helper.CmdShouldPass("odo", "url", "create", url1, "--port", "3000", "--host", host, "--secure", "--ingress")
			helper.CmdShouldPass("odo", "push", "--project", namespace)
			stdout = helper.CmdShouldPass("odo", "url", "describe", url1)
			helper.MatchAllInOutput(stdout, []string{url1 + "." + host, "Pushed", "true", "ingress"})
		})
	})

	Context("Testing URLs for OpenShift specific scenarios", func() {
		JustBeforeEach(func() {
			if os.Getenv("KUBERNETES") == "true" {
				Skip("This is a OpenShift specific scenario, skipping")
			}
		})

		It("should error out when a host is provided with a route on a openShift cluster", func() {
			url1 := helper.RandString(5)

			helper.CmdShouldPass("odo", "create", "nodejs", "--project", namespace, componentName)

			helper.CopyExample(filepath.Join("source", "devfiles", "nodejs", "project"), context)
			helper.CopyExampleDevFile(filepath.Join("source", "devfiles", "nodejs", "devfile.yaml"), filepath.Join(context, "devfile.yaml"))

			output := helper.CmdShouldFail("odo", "url", "create", url1, "--host", "com")
			Expect(output).To(ContainSubstring("host is not supported"))
		})

		It("should list route and ingress urls with appropriate state", func() {
			url1 := helper.RandString(5)
			url2 := helper.RandString(5)
			ingressurl := helper.RandString(5)
			host := helper.RandString(5) + ".com"

			helper.CmdShouldPass("odo", "create", "nodejs", "--project", namespace, componentName)

			helper.CopyExample(filepath.Join("source", "devfiles", "nodejs", "project"), context)
			helper.CopyExampleDevFile(filepath.Join("source", "devfiles", "nodejs", "devfile.yaml"), filepath.Join(context, "devfile.yaml"))

			helper.CmdShouldPass("odo", "url", "create", url1, "--port", "3000", "--secure")
			helper.CmdShouldPass("odo", "url", "create", ingressurl, "--port", "3000", "--host", host, "--ingress")
			helper.CmdShouldPass("odo", "push", "--project", namespace)
			helper.CmdShouldPass("odo", "url", "create", url2, "--port", "3000")
			stdout := helper.CmdShouldPass("odo", "url", "list", "--context", context)
			helper.MatchAllInOutput(stdout, []string{url1, "Pushed", "true", "route"})
			helper.MatchAllInOutput(stdout, []string{url2, "Not Pushed", "false", "route"})
			helper.MatchAllInOutput(stdout, []string{ingressurl, "Pushed", "false", "ingress"})

			helper.CmdShouldPass("odo", "url", "delete", url1, "-f")
			stdout = helper.CmdShouldPass("odo", "url", "list", "--context", context)
			helper.MatchAllInOutput(stdout, []string{url1, "Locally Deleted", "true", "route"})
			helper.MatchAllInOutput(stdout, []string{url2, "Not Pushed", "false", "route"})
			helper.MatchAllInOutput(stdout, []string{ingressurl, "Pushed", "false", "ingress"})
		})

		It("should create a automatically route on a openShift cluster", func() {
			url1 := helper.RandString(5)

			helper.CmdShouldPass("odo", "create", "nodejs", "--project", namespace, componentName)

			helper.CopyExample(filepath.Join("source", "devfiles", "nodejs", "project"), context)
			helper.CopyExampleDevFile(filepath.Join("source", "devfiles", "nodejs", "devfile.yaml"), filepath.Join(context, "devfile.yaml"))

			helper.CmdShouldPass("odo", "url", "create", url1)

			helper.CmdShouldPass("odo", "push", "--namespace", namespace)
			pushStdOut := helper.CmdShouldPass("odo", "push", "--namespace", namespace)
			helper.DontMatchAllInOutput(pushStdOut, []string{"successfully deleted", "created"})
			Expect(pushStdOut).To(ContainSubstring("URLs are synced with the cluster, no changes are required"))

			output := helper.CmdShouldPass("oc", "get", "routes", "--namespace", namespace)
			Expect(output).Should(ContainSubstring(url1))

			helper.CmdShouldPass("odo", "url", "delete", url1, "-f")
			helper.CmdShouldPass("odo", "push", "--namespace", namespace)
			pushStdOut = helper.CmdShouldPass("odo", "push", "--namespace", namespace)
			helper.DontMatchAllInOutput(pushStdOut, []string{"successfully deleted", "created"})
			Expect(pushStdOut).To(ContainSubstring("URLs are synced with the cluster, no changes are required"))

			output = helper.CmdShouldPass("oc", "get", "routes", "--namespace", namespace)
			Expect(output).ShouldNot(ContainSubstring(url1))
		})

		It("should describe appropriate Route URLs", func() {
			url1 := helper.RandString(5)

			helper.CmdShouldPass("odo", "create", "nodejs", "--project", namespace, componentName)

			helper.CopyExample(filepath.Join("source", "devfiles", "nodejs", "project"), context)
			helper.CopyExampleDevFile(filepath.Join("source", "devfiles", "nodejs", "devfile.yaml"), filepath.Join(context, "devfile.yaml"))

			helper.CmdShouldPass("odo", "url", "create", url1, "--port", "3000")

			stdout := helper.CmdShouldPass("odo", "url", "describe", url1)
			helper.MatchAllInOutput(stdout, []string{url1, "Not Pushed", "false", "route", "odo push"})

			helper.CmdShouldPass("odo", "push", "--project", namespace)
			stdout = helper.CmdShouldPass("odo", "url", "describe", url1)
			helper.MatchAllInOutput(stdout, []string{url1, "Pushed", "false", "route"})
			helper.CmdShouldPass("odo", "url", "delete", url1, "-f")
			stdout = helper.CmdShouldPass("odo", "url", "describe", url1)
			helper.MatchAllInOutput(stdout, []string{url1, "Locally Deleted", "false", "route"})

			helper.CmdShouldPass("odo", "url", "create", url1, "--port", "3000", "--secure")
			helper.CmdShouldPass("odo", "push", "--project", namespace)
			stdout = helper.CmdShouldPass("odo", "url", "describe", url1)
			helper.MatchAllInOutput(stdout, []string{url1, "Pushed", "true", "route"})
		})
	})
})
