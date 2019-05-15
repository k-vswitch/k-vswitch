IMAGE_REPO=andrewsykim/kube-ovs
IMAGE_TAG=v1

all: compile build push

.PHONY: compile
compile:
	docker run \
	  -v $(PWD):/go/src/github.com/kube-ovs/kube-ovs \
	  -w /go/src/github.com/kube-ovs/kube-ovs \
	  golang:1.12 sh -c '\
	  CGO_ENABLED=0 GOOS=linux GO111MODULE=on go build -mod=vendor \
	  github.com/kube-ovs/kube-ovs/cmd/kube-ovs'
	docker run \
	  -v $(PWD):/go/src/github.com/kube-ovs/kube-ovs \
	  -w /go/src/github.com/kube-ovs/kube-ovs \
	  golang:1.12 sh -c '\
	  CGO_ENABLED=0 GOOS=linux GO111MODULE=on go build -mod=vendor \
	  github.com/kube-ovs/kube-ovs/cmd/kube-ovs-controller'
	docker run \
	  -v $(PWD):/go/src/github.com/kube-ovs/kube-ovs \
	  -w /go/src/github.com/kube-ovs/kube-ovs \
	  golang:1.12 sh -c '\
	  CGO_ENABLED=0 GOOS=linux GO111MODULE=on go build -mod=vendor \
	  github.com/kube-ovs/kube-ovs/cmd/kube-ovs-cni'

.PHONY: build
build:
	docker build -t $(IMAGE_REPO):$(IMAGE_TAG) .
	docker tag $(IMAGE_REPO):$(IMAGE_TAG) $(IMAGE_REPO):latest

.PHONY: push
push:
	docker push $(IMAGE_REPO):$(IMAGE_TAG)
	docker push $(IMAGE_REPO):latest
