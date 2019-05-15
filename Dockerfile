FROM debian:stretch-slim

RUN apt update && apt install -y openvswitch-switch iptables

ADD k-vswitchd /bin
ADD k-vswitch-cni /bin
ADD k-vswitch-controller /bin

CMD ["/bin/k-vswitchd"]
