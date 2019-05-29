FROM debian:stretch-slim

RUN apt update && apt install -y openvswitch-switch iptables

ADD bin/k-vswitchd /bin
ADD bin/k-vswitch-cni /bin
ADD bin/k-vswitch-controller /bin

CMD ["/bin/k-vswitchd"]
