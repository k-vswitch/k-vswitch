IMAGE_REPO=kvswitch/k-vswitch
IMAGE_TAG=latest

all: compile build push

.PHONY: compile
compile:
	docker run \
	  -v $(PWD):/go/src/github.com/k-vswitch/k-vswitch \
	  -w /go/src/github.com/k-vswitch/k-vswitch \
	  golang:1.12 sh -c '\
	  CGO_ENABLED=0 GOOS=linux GO111MODULE=on go build -mod=vendor \
	  github.com/k-vswitch/k-vswitch/cmd/k-vswitchd'
	docker run \
	  -v $(PWD):/go/src/github.com/k-vswitch/k-vswitch \
	  -w /go/src/github.com/k-vswitch/k-vswitch \
	  golang:1.12 sh -c '\
	  CGO_ENABLED=0 GOOS=linux GO111MODULE=on go build -mod=vendor \
	  github.com/k-vswitch/k-vswitch/cmd/k-vswitch-controller'
	docker run \
	  -v $(PWD):/go/src/github.com/k-vswitch/k-vswitch \
	  -w /go/src/github.com/k-vswitch/k-vswitch \
	  golang:1.12 sh -c '\
	  CGO_ENABLED=0 GOOS=linux GO111MODULE=on go build -mod=vendor \
	  github.com/k-vswitch/k-vswitch/cmd/k-vswitch-cni'

.PHONY: build
build:
	docker build -t $(IMAGE_REPO):$(IMAGE_TAG) .
	docker tag $(IMAGE_REPO):$(IMAGE_TAG) $(IMAGE_REPO):latest

.PHONY: push
push:
	docker push $(IMAGE_REPO):$(IMAGE_TAG)
	docker push $(IMAGE_REPO):latest
