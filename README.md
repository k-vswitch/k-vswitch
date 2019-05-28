# k-vswitch

k-vswitch is an easy-to-operate and high performance Kubernetes networking plugin based on [Open vSwitch](https://www.openvswitch.org/).

**WARNING**: k-vswitch is in active development and is not advised for production environments. There will be a production-ready release in the future.

## Installation

### Components

k-vswitch has 3 core components:

* **k-vswitch-cni**: a CNI implementation based on OVS - adds pod network namespaces as OVS ports.
                     k-vswitch-cni is automatically installed into nodes by k-vswitchd.
* **k-vswitchd**: a DaemonSet that runs on your cluster, responsible for setting up the OVS bridge and any necessary flows.
* **k-vswitch-controller**: a StatefulSet responsible for managing CRDs that are consumed by k-vswitchd.

### Requirements

The only install requirement for your cluster is that every Kubernetes node has Open vSwitch installed.
Depending on your Linux distribution, you can install it in the following ways:

```bash
# debian / ubuntu
$ apt-get install openvswitch-switch
# or...
$ apt install openvswitch-switch


# fedora / centos / RHEL
$ yum install openvswitch
# or...
$ dnf install openvswitch
```

### Install Steps

k-vswitch requires the following information before it can be installed:

* **cluster CIDR**: this is the same cluster CIDR you configured on various components of your cluster via `--cluster-cidr`.
* **service CIDR**: this is the same service CIDR you configured on the `kube-apiserver` via `--service-cluster-ip-range`.
* **overlay type**: this is the overlay type to use, currently 'vxlan' and 'gre' are supported. 'gre' is recommended but some
           cloud providers may not support it in which case you can use `vxlan`.

Once you have the following, you can install the latest deployment spec, update the k-vswitch configmap based on the above
parameters and apply it to your cluster:

```bash
$ curl -LO https://raw.githubusercontent.com/k-vswitch/k-vswitch/master/deployment/k-vswitch-latest.yaml
$ vim k-vswitch-latest.yaml  # edit the first ConfigMap on this file based on your cluster configuration
$ kubectl apply -f k-vswitch-latest.yaml
```

## Upcoming features

k-vswitch is in active development with the following features planned for the near-future:

* **Network Policies support** - allow/drop packets based on the Kubernetes NetworkPolicy API
* **Windows support** - k-vswitch is currently only supported on Linux. Since OVS supports Windows, k-vswitch will support it in the future.
