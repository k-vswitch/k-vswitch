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
`--allocate-node-cidrs=true` flag on `kube-controller-manager` and a cluster CIDR using `--cluster-cidr=<your-cluster-cidr>`.

If you are using `kubeadm`, you can enable this functionality with:

```bash
$ kubeadm init --pod-network-cidr=<your-cluster-cidr>
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

## Features

### Network Policies

k-vswitch supports [Kubernetes Network Policies](https://kubernetes.io/docs/concepts/services-networking/network-policies/) by programming flows on Open vSwitch
which then matches ingressing/egressing packets on the bridge and allows/denies them as specified in the network policy API.

### Performance

k-vswitch is performant by nature due to the Open vSwitch Linux Kernel Datapath. The "fast-path" kernel module allows the kernel to
cache subsequent packets in kernel-space, significantly increasing performance compared to the standard Linux bridge, especially in
high throughput environments. You can learn more about OVS performance in [this blog post](https://networkheresy.com/2014/11/13/accelerating-open-vswitch-to-ludicrous-speed/) or the [OVS white paper](https://www.usenix.org/system/files/conference/nsdi15/nsdi15-paper-pfaff.pdf).

### GRE / VxLAN Overlay

k-vswitch supports GRE and VxLAN overlay for your Kubernetes cluster. GRE is recommended, however, some cloud providers do not allow
GRE traffic over VM network so you may need to use VxLAN in that case.

## Upcoming features

k-vswitch is in active development with the following features planned for the near-future:

* **Windows support** - k-vswitch is currently only supported on Linux. Since OVS supports Windows, k-vswitch may support it in the future.
* **Direct Routing** - integrate with other networking plugins like kube-router to do BGP direct routing to pods along with k-vswitch (i.e. no overlay)
