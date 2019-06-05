# k-vswitch

k-vswitch is an easy-to-operate, high performance and secure Kubernetes networking plugin based on [Open vSwitch](https://www.openvswitch.org/).

**WARNING**: k-vswitch is in active development and is not advised for production environments. There will be a production-ready release in the future.

Table of Contents
=================

   * [k-vswitch](#k-vswitch)
   * [Table of Contents](#table-of-contents)
      * [Features](#features)
         * [Network Policies](#network-policies)
         * [Performance](#performance)
         * [GRE / VxLAN Overlay](#gre--vxlan-overlay)
         * [ARP Responder](#arp-responder)
      * [Architecture](#architecture)
         * [Components](#components)
            * [k-vswitch-cni](#k-vswitch-cni)
            * [k-vswitchd](#k-vswitchd)
            * [k-vswitch-controller](#k-vswitch-controller)
         * [Architecture Diagram](#architecture-diagram)
      * [Installation](#installation)
         * [Requirements](#requirements)
            * [OVS Installed on Kubernetes Nodes](#ovs-installed-on-kubernetes-nodes)
            * [kube-controller-manager IPAM enabled](#kube-controller-manager-ipam-enabled)
         * [Install Steps](#install-steps)
      * [Upcoming features](#upcoming-features)

## Features

### Network Policies

k-vswitch supports [Kubernetes Network Policies](https://kubernetes.io/docs/concepts/services-networking/network-policies/) by programming flows on Open vSwitch
which then matches ingressing/egressing packets on the bridge and only allows packages as specifed by the network policy API.

### Performance

k-vswitch is performant by nature due to the Open vSwitch Kernel Datapath. The "fast-path" kernel module allows the kernel to
cache subsequent packets in kernel-space, significantly increasing performance compared to the standard Linux bridge, especially in
high throughput environments. You can learn more about OVS performance in [this blog post](https://networkheresy.com/2014/11/13/accelerating-open-vswitch-to-ludicrous-speed/) or the [OVS white paper](https://www.usenix.org/system/files/conference/nsdi15/nsdi15-paper-pfaff.pdf).

### GRE / VxLAN Overlay

k-vswitch supports GRE and VxLAN overlay for your Kubernetes cluster. GRE is recommended, however, some cloud providers do not allow
GRE traffic over VM network so you may need to use VxLAN in that case.

### ARP Responder

k-vswitch programs flows on the Open vSwitch bridge to automatically send ARP replies to all ARP requests on your pod network. This removes
the need for L2 broadcast/learning for pods on your cluster.


See [Upcoming features](#upcoming-features) for a list of future improvements/features planned for k-vswitch.

## Architecture

### Components

k-vswitch has 3 core components:

#### k-vswitch-cni

`k-vswitch-cni` is a CNI implemenation based on OVS. It receives a pod's network namespace from the `kubelet`, creates a veth pair with one end being the pod network namespace
and the other as a port on the OVS bridge. Port information for a given pod is then automatically stored into `ovsdb-server` (automatically installed with Open vSwitch)
so that `k-vswitchd` can fetch information about the OVS port later.

`k-vswitch-cni` is automatically installed and setup by `k-vswitchd` at startup. More on `k-vswitchd` below.

#### k-vswitchd

`k-vswitchd` runs as a Kubernetes DaemonSet on your cluster, it is responsible for:
* Installing and configuring `k-vswitch-cni` on the node
* Configuring the OVS bridge (`k-vswitch0`) and adding the necessary interfaces/routes associated with the bridge on the host.
* Programming flows on the OVS bridge based on the state of the cluster by watching the apiserver for pods, network policies and the `VSwitchConfig` CRD (more on this below).

#### k-vswitch-controller

`k-vswitch-controller` is a StatefulSet (with repliacs=1) responsible for managing the `VSwitchConfig` CRD. The `VSwitchConfig` CRD is responsible for holding information
required by `k-vswitchd` in order to configure the OVS bridge. There should always be 1 `VSwitchConfig` object associated with a node in your cluster. The name of the `VSwitchConfig`
resource should equal the name of the node.

As of today, the `VSwitchConfig` resource holds the following information:
* cluster CIDR - the pod networking CIDR for the entire cluster
* pod CIDR - the pod CIDR allocated to this specific vSwitch/Node
* service CIDR - the service proxy CIDR on the cluster
* overlay type - the overlay protocol to use, `vxlan` or `gre`
* overlay IP - the private address on the node to use for the overlay tunnel

The motivation behind the `VSwitchConfig` CRD is to consolidate all the necessary information for an OVS bridge into a single resource. This significantly simplies
the controllers running in `k-vswitchd` and the number of places that requires user input. Some of the above however, (like cluster CIDR) require user input,
more on this in [Installation](#installation).

### Architecture Diagram

![k-vswitch-overview-diagram](/docs/images/k-vswitch-overview-diagram.png "k-vswitch High Level Overview")

## Installation

A large focus on k-vswitch's design is ease of use and installation. If you find that k-vswitch is hard to operate and install, please
file an issue and offer your feedback!

### Requirements

#### OVS Installed on Kubernetes Nodes

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

#### kube-controller-manager IPAM enabled

Ensure that the `kube-controller-manager` is configured to allocate pod CIDRs for your nodes. You can enable this by setting the
`--allocate-node-cidrs=true` flag and a cluster CIDR using the `--cluster-cidr=<your-cluster-cidr>` flag.

If you are using `kubeadm`, you can enable this functionality with:

```bash
$ kubeadm init --pod-network-cidr=<your-cluster-cidr>
```

### Install Steps

k-vswitch requires the following cluster parameters before it can be installed:

* **cluster CIDR**: this is the same cluster CIDR you configured on various components of your cluster via `--cluster-cidr`.
* **service CIDR**: this is the same service CIDR you configured on the `kube-apiserver` via `--service-cluster-ip-range`.
* **overlay type**: this is the overlay type to use, currently `vxlan` and `gre` are supported. `gre` is recommended but some
           cloud providers may not support it in which case you can use `vxlan`.

Once you have the following parameters, you can download the latest deployment spec. The first resource on the deployment spec should be a ConfigMap.
Update the ConfigMap fields based on the above parameters and apply it to your cluster:

```bash
$ curl -LO https://raw.githubusercontent.com/k-vswitch/k-vswitch/master/deployment/k-vswitch-latest.yaml
$ vim k-vswitch-latest.yaml  # edit the first ConfigMap on this file based on your cluster configuration
$ kubectl apply -f k-vswitch-latest.yaml
```

## Upcoming features

k-vswitch is in active development with the following features planned for the near-future:

* **Native Pod Routing** - integrate with kube-router to do BGP native routing to pods along with k-vswitch (i.e. no overlay)
* **Service Proxy** - implement kube-proxy like load balancing for Kubernetes Services via OpenFlow
* **Pod Traffic Telemetry** - traffic telemetry for pods using sFlow on Open vSwitch
* **Windows support** - k-vswitch is currently only supported on Linux. Since OVS supports Windows, k-vswitch may support it in the future.
