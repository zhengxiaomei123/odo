#!/bin/sh

# fail if some commands fails
set -e
# show commands
set -x

export CI="openshift"
make configure-installer-tests-cluster
make bin
mkdir -p $GOPATH/bin
go get -u github.com/onsi/ginkgo/ginkgo
export PATH="$PATH:$(pwd):$GOPATH/bin"
export ARTIFACTS_DIR="/tmp/artifacts"
export CUSTOM_HOMEDIR=$ARTIFACTS_DIR
export ODO_BOOTSTRAPPER_IMAGE=registry.redhat.io/ocp-tools-4/odo-init-container-rhel8:1.1.4

# Copy kubeconfig to temporary kubeconfig file
# Read and Write permission to temporary kubeconfig file
TMP_DIR=$(mktemp -d)
cp $KUBECONFIG $TMP_DIR/kubeconfig
chmod 640 $TMP_DIR/kubeconfig
export KUBECONFIG=$TMP_DIR/kubeconfig

# Login as developer
odo login -u developer -p developer

# Check login user name for debugging purpose
oc whoami

# Integration tests
make test-generic
make test-cmd-link-unlink
make test-cmd-pref-config
make test-cmd-watch
make test-cmd-debug
make test-cmd-login-logout
make test-cmd-project
make test-cmd-app
make test-cmd-storage
make test-cmd-push
make test-cmd-watch

# E2e tests
make test-e2e-beta

odo logout
