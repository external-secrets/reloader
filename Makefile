IMG ?= ghcr.io/external-secrets/reloader
ENVTEST_K8S_VERSION = 1.35.0

ARCH ?= amd64 arm64
BUILD_ARGS ?= CGO_ENABLED=0
DOCKER_BUILD_ARGS ?=
DOCKERFILE ?= Dockerfile
OUTPUT_DIR  ?= bin
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif
# ====================================================================================
# Logger

TIME_LONG	= `date +%Y-%m-%d' '%H:%M:%S`
TIME_SHORT	= `date +%H:%M:%S`
TIME		= $(TIME_SHORT)

INFO	= echo ${TIME} ${BLUE}[ .. ]${CNone}
WARN	= echo ${TIME} ${YELLOW}[WARN]${CNone}
ERR		= echo ${TIME} ${RED}[FAIL]${CNone}
OK		= echo ${TIME} ${GREEN}[ OK ]${CNone}
FAIL	= (echo ${TIME} ${RED}[FAIL]${CNone} && false)
# ============================================================
CONTAINER_TOOL ?= docker

SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

#Tags
ifeq ($(shell git tag),)
export VERSION := $(shell echo "v0.0.0-$$(git rev-list HEAD --count)-g$$(git describe --dirty --always)" | sed 's/-/./2' | sed 's/-/./2')
else
export VERSION := $(shell git describe --dirty --always --tags --exclude 'helm*' | sed 's/-/./2' | sed 's/-/./2')
endif

TAG_SUFFIX ?=
export IMAGE_TAG ?= $(VERSION)$(TAG_SUFFIX)

CHART_DIR ?= deploy/charts/reloader

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=$(CHART_DIR)/crds

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests generate fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter
	$(GOLANGCI_LINT) run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	$(GOLANGCI_LINT) run --fix

##@ Build

.PHONY: build
build: $(addprefix build-,$(ARCH)) ## Build binary

.PHONY: build-%
build-%: manifests generate fmt vet ## Build binary for the specified arch
	@$(INFO) go build $*
	$(BUILD_ARGS) GOOS=linux GOARCH=$* \
		go build -o '$(OUTPUT_DIR)/reloader-linux-$*' cmd/main.go
	@$(OK) go build $*

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./cmd/main.go

.PHONY: docker.tag
docker.tag:  ## Emit IMAGE_TAG
	@echo $(IMAGE_TAG)

.PHONY: docker.build
docker.build: $(addprefix build-,$(ARCH)) ## Build the docker image
	@$(INFO) docker buildx build
	echo docker buildx build -f $(DOCKERFILE) . $(DOCKER_BUILD_ARGS) -t ${IMG}:${IMAGE_TAG}
	docker buildx build -f $(DOCKERFILE) . $(DOCKER_BUILD_ARGS) -t ${IMG}:${IMAGE_TAG}
	@$(OK) docker buildx build


.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push ${IMG}


##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests ## Install the Helm chart into the K8s cluster specified in ~/.kube/config.
	$(HELM) upgrade --install reloader $(CHART_DIR) \
		--namespace external-secrets-reloader --create-namespace \
		--set image.repository=$(IMG) \
		--set image.tag=$(IMAGE_TAG)

.PHONY: uninstall
uninstall: ## Uninstall the Helm chart from the K8s cluster specified in ~/.kube/config.
	$(HELM) uninstall reloader --namespace external-secrets-reloader --ignore-not-found=$(ignore-not-found)

.PHONY: helm.lint
helm.lint: ## Lint the Helm chart using chart-testing.
	ct lint --config=.github/ci/ct.yaml --charts $(CHART_DIR)

.PHONY: helm.template
helm.template: manifests ## Render Helm chart templates locally.
	$(HELM) template reloader $(CHART_DIR) \
		--namespace external-secrets-reloader \
		--set image.repository=$(IMG) \
		--set image.tag=$(IMAGE_TAG)

##@ API Spec
.PHONY: spec-generate
spec-generate: ## generate api reference documentation to go to the website
	./hack/generate.sh docs/api/spec.md


##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUBECTL ?= kubectl
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint
HELM ?= helm

## Tool Versions
CONTROLLER_TOOLS_VERSION ?= v0.20.1
ENVTEST_VERSION ?= release-0.23
GOLANGCI_LINT_VERSION ?= v2.11.3

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(LOCALBIN) $(GOLANGCI_LINT_VERSION)


# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f $(1) || true ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv $(1) $(1)-$(3) ;\
} ;\
ln -sf $(1)-$(3) $(1)
endef
