
.DEFAULT_GOAL:=help

VERSION_SUFFIX ?= -dev
PROD_VERSION ?= 0.7.4${VERSION_SUFFIX}
PROD_BUILD_ID ?= latest

STATIC_IP_IMG ?= "gcr.io/spectro-common-dev/${USER}/capv-static-ip:latest"
OVERLAY ?= base

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"
COVER_DIR=_build/cov
MANIFEST_DIR=_build/manifests
export CURRENT_DIR=${CURDIR}


all: generate manifests static bin


# static analysis section
static: fmt vet lint ## Run static code analysis
fmt: ## Run go fmt against code
	go fmt ./...
vet: ## Run go vet against code
	go vet ./...
lint: ## Run golangci-lint  against code
	golangci-lint run    ./...  --timeout 10m  --tests=false

# Run tests
test: generate fmt vet manifests
	go test ./... -coverprofile cover.out

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	go run ./main.go

# Install CRDs into a cluster
install: manifests
	kustomize build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests
	kustomize build config/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	cd config/manager && kustomize edit set image controller=${STATIC_IP_IMG}
	kustomize build config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	@mkdir -p $(MANIFEST_DIR)
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases
	cd config/manager && kustomize edit set image controller=${STATIC_IP_IMG}
	kustomize build config/default > $(MANIFEST_DIR)/staticip-manifest.yaml


# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

bin: generate ## Generate binaries
	go build -o bin/manager main.go

# Build the docker image
docker-build:
	docker build . -t ${STATIC_IP_IMG}

# Push the docker image
docker-push:
	docker push ${STATIC_IP_IMG}

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.5 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif
