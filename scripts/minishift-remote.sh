#!/bin/sh

OPENSHIFT_CLIENT_BINARY_URL=${OPENSHIFT_CLIENT_BINARY_URL:-'https://github.com/openshift/origin/releases/download/v3.11.0/openshift-origin-client-tools-v3.11.0-0cbc58b-linux-64bit.tar.gz'}
MINISHIFT_ARCHIVE_URL=${MINISHIFT_ARCHIVE_URL:-'https://github.com/minishift/minishift/releases/download/v1.30.0/minishift-1.30.0-linux-amd64.tgz'}

ssh-keygen -t rsa -N "" -f ~/.ssh/id_rsa
ls ~/.ssh/

sudo systemctl start docker
sudo systemctl status docker
sudo systemctl start firewalld
sudo systemctl status firewalld
ssh-keyscan localhost >> ~/.ssh/known_hosts
cp ~/.ssh/id_rsa.pub ~/.ssh/authorized_keys 

## download oc binaries
sudo wget $OPENSHIFT_CLIENT_BINARY_URL -O /tmp/openshift-origin-client-tools.tar.gz 2> /dev/null > /dev/null

sudo tar -xvzf /tmp/openshift-origin-client-tools.tar.gz --strip-components=1 -C /usr/local/bin
## Get oc version
oc version

## download minishift binaries
sudo wget $MINISHIFT_ARCHIVE_URL -O /tmp/minishift.tgz 2> /dev/null > /dev/null

sudo tar -xvzf /tmp/minishift.tgz --strip-components=1 -C /usr/local/bin

## Get minishift version
minishift version

if [ "$1" = "service-catalog" ]; then
   MINISHIFT_ENABLE_EXPERIMENTAL=y minishift start --vm-driver generic --remote-ipaddress localhost --remote-ssh-user `whoami` --remote-ssh-key ~/.ssh/id_rsa --extra-clusterup-flags "--enable=*,service-catalog,automation-service-broker" 
  minishift status
else
  minishift start --vm-driver generic --remote-ipaddress localhost --remote-ssh-user `whoami` --remote-ssh-key ~/.ssh/id_rsa
  echo "^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^"
  minishift status
fi  

