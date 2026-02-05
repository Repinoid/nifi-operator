IMAGE_REPO ?= naeel
TAG ?= latest
PLATFORM ?= linux/amd64

.PHONY: build-all push-all build-operator push-operator build-registry push-registry tidy

tidy:
	go mod tidy

build-all: build-operator build-registry
push-all: push-operator push-registry

build-operator:
	docker build --platform $(PLATFORM) -t $(IMAGE_REPO)/terraform-registry-operator:$(TAG) -f build/Dockerfile.operator .

push-operator:
	docker push $(IMAGE_REPO)/terraform-registry-operator:$(TAG)

build-registry:
	docker build --platform $(PLATFORM) -t $(IMAGE_REPO)/terraform-registry-server:$(TAG) -f build/Dockerfile.registry .

push-registry:
	docker push $(IMAGE_REPO)/terraform-registry-server:$(TAG)
