# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.27.1

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test `go list ./... | grep -v cmd` -coverpkg=./... -coverprofile=coverage.out

.PHONY: buildx
buildx: fmt vet
	docker buildx build --platform linux/amd64 -t serverless-registry.cn-shanghai.cr.aliyuncs.com/opensource/test/module_controller:latest .

.PHONY: build
build: fmt vet
	docker build -t serverless-registry.cn-shanghai.cr.aliyuncs.com/opensource/test/module_controller:latest .

.PHONY: minikube-delete
minikube-delete: ## Delete module-controller deployment from minikube
	kubectl delete deployments.apps/module-controller || true
	kubectl wait --for=delete pod -l app=module-controller --timeout=90s

.PHONY: minikube-debug-build
minikube-debug-build: fmt vet minikube-delete ## Build and deploy debug version using minikube
	minikube image rm serverless-registry.cn-shanghai.cr.aliyuncs.com/opensource/test/module-controller-v2:latest
	minikube image build -f debug.Dockerfile -t serverless-registry.cn-shanghai.cr.aliyuncs.com/opensource/test/module-controller-v2:latest .
	kubectl apply -f example/quick-start/module-controller-test.yaml
	kubectl wait --for=condition=available --timeout=90s deployments/module-controller

.PHONY: minikube-debug
minikube-debug:
	kubectl exec deployments/module-controller -it -- dlv --listen=:2345 --headless=true --api-version=2 --accept-multiclient exec ./module_controller

.PHONY: minikube-port-forward
minikube-port-forward:
	kubectl port-forward deployments/module-controller 2345:2345

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUBECTL ?= kubectl
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest

## Tool Versions
KUSTOMIZE_VERSION ?= v5.0.1
CONTROLLER_TOOLS_VERSION ?= v0.12.0

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary. If wrong version is installed, it will be removed before downloading.
$(KUSTOMIZE): $(LOCALBIN)
	@if test -x $(LOCALBIN)/kustomize && ! $(LOCALBIN)/kustomize version | grep -q $(KUSTOMIZE_VERSION); then \
		echo "$(LOCALBIN)/kustomize version is not expected $(KUSTOMIZE_VERSION). Removing it before installing."; \
		rm -rf $(LOCALBIN)/kustomize; \
	fi
	test -s $(LOCALBIN)/kustomize || GOBIN=$(LOCALBIN) GO111MODULE=on go install sigs.k8s.io/kustomize/kustomize/v5@$(KUSTOMIZE_VERSION)

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary. If wrong version is installed, it will be overwritten.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen && $(LOCALBIN)/controller-gen --version | grep -q $(CONTROLLER_TOOLS_VERSION) || \
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

