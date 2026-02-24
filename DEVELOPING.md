# Developing the Reloader Controller

This guide covers how to set up your environment, generate manifests, install CRDs, run the controller locally or with Tilt, and run tests.

## Prerequisites

- **Go** 1.22+ (1.25.x is used in CI; [asdf](https://asdf-vm.com/) or official install)
- **kubectl** configured against a Kubernetes cluster (e.g. kind, minikube, or cloud)
- **Docker** (for building images and for Tilt)
- **Tilt** (optional, for live reload): [install Tilt](https://docs.tilt.dev/install.html)

### Go environment (asdf / GOROOT)

If you use asdf or a non-default Go install, ensure `GOROOT` is not set to a missing path (e.g. `/usr/local/go`). Otherwise `make` targets that run `go install` (e.g. controller-gen) can fail.

```bash
# Use your Go (e.g. asdf) and avoid a stale GOROOT
export PATH="$HOME/.asdf/shims:$PATH"
unset GOROOT
```

For **controller-gen** to build with Go 1.25, the Makefile uses `CONTROLLER_TOOLS_VERSION=v0.20.0`. If you need to install tools again with a different Go version, you can override:

```bash
CONTROLLER_TOOLS_VERSION=v0.20.0 make manifests
```

---

## Generating manifests and code

Manifests (CRDs, RBAC, webhook config) and generated code (DeepCopy, etc.) are produced by controller-gen and must be regenerated after API changes.

### Generate everything

```bash
make manifests   # CRDs, RBAC, webhook under config/
make generate    # DeepCopy and similar under api/
```

Or run both before building or testing:

```bash
make manifests generate
```

### Regenerate after API changes

After editing types under `api/v1alpha1/` (e.g. new fields, new CRD):

1. Run `make manifests` so CRDs and RBAC in `config/` are updated.
2. Run `make generate` if you added or changed types that need DeepCopy.
3. Commit the updated files under `config/crd/bases/`, `config/rbac/`, and `api/` (e.g. `zz_generated.deepcopy.go`).

---

## Installing CRDs

CRDs are installed into the cluster pointed at by your `KUBECONFIG` (e.g. `~/.kube/config`).

### Install CRDs only

Uses kustomize to build `config/crd` and applies the result:

```bash
make install
```

This installs the `Config` CRD (and any other CRDs under `config/crd/bases/`) into the cluster. It does **not** deploy the controller.

### Uninstall CRDs

```bash
make uninstall
```

To avoid errors when CRDs or resources are already deleted:

```bash
make uninstall ignore-not-found=true
```

---

## Running the controller

### Option 1: Run locally (outside the cluster)

Runs the controller on your machine; it uses your kubeconfig to talk to the cluster. No image build required.

```bash
make run
```

This runs `go run ./cmd/main.go` after `manifests generate fmt vet`. Ensure CRDs are installed first (`make install`).

### Option 2: Deploy with Kustomize (in-cluster)

Builds the controller image and deploys the full stack (CRDs, RBAC, deployment, services) from `config/default`:

```bash
# Build and load image (example for kind)
make docker.build
kind load docker-image ghcr.io/external-secrets-inc/reloader:$(make -s docker.tag) --name <your-kind-cluster>

# Deploy (CRDs + RBAC + controller)
make deploy
```

`make deploy` runs `kustomize build config/default | kubectl apply -f -`. The controller runs in the `external-secrets-reloader` namespace with name prefix `reloader-`.

To remove the deployment (but not CRDs):

```bash
make undeploy
```

### Option 3: Tilt (live reload in-cluster)

Tilt builds the controller binary, builds a small image from `tilt.dockerfile`, and deploys the same stack as `config/default` with live-update: code changes rebuild the binary and sync into the running container so you don’t need to rebuild the image for every change.

1. **Ensure cluster and CRDs**

   Use a local cluster (e.g. kind, minikube) and install CRDs if they are not already present:

   ```bash
   make install
   ```

2. **Start Tilt**

   From the repo root:

   ```bash
   tilt up
   ```

   Tilt will:

   - Run `kustomize build config/default` and apply the result (namespace `external-secrets-reloader`, deployment `reloader-controller-manager`, etc.).
   - Build the Go binary (`bin/reloader`), then build the image from `tilt.dockerfile` and deploy it.
   - On file changes under `api/`, `cmd/`, `internal/`, `pkg/`, sync the new binary into the container and restart the process.

3. **Optional: debug with Delve**

   Create `tilt-settings.yaml` in the repo root:

   ```yaml
   debug:
     enabled: true
   ```

   Then run `tilt up`. Tilt will use `tilt.debug.dockerfile` and forward port 30000 for the Delve debugger.

4. **Stop**

   In the Tilt UI or in the terminal where you ran `tilt up`, stop with `Ctrl+C` or via the Tilt UI. To remove the deployed resources, run `tilt down` or:

   ```bash
   make undeploy
   ```

---

## Testing

### Unit / integration tests

Runs tests (excluding e2e) using envtest for a temporary API server:

```bash
make test
```

This depends on `manifests`, `generate`, `fmt`, `vet`, and `envtest`. The first run may download envtest and kubebuilder assets.

### Linting

```bash
make lint        # Run golangci-lint
make lint-fix    # Run with --fix
```

### Format and vet

```bash
make fmt
make vet
```

---

## Quick reference

| Task              | Command                    |
|-------------------|----------------------------|
| Generate CRDs     | `make manifests`           |
| Generate DeepCopy | `make generate`            |
| Install CRDs      | `make install`             |
| Uninstall CRDs    | `make uninstall`           |
| Run locally       | `make run`                 |
| Deploy to cluster | `make deploy`              |
| Undeploy          | `make undeploy`            |
| Run tests         | `make test`                |
| Live reload       | `tilt up`                  |

---

## Project layout (relevant to development)

- **`api/v1alpha1/`** – API types and CRD definitions (edit here; then `make manifests generate`).
- **`config/crd/bases/`** – Generated CRD YAML; do not edit by hand.
- **`config/default/`** – Kustomize overlay used for deploy and Tilt (CRDs + RBAC + manager + metrics).
- **`config/manager-local/`** – Deployment and services for the controller.
- **`config/rbac/`** – ServiceAccount, Role, RoleBinding (and related); partially generated from controller RBAC markers.
- **`cmd/main.go`** – Entrypoint; wires scheme, manager, and reconciler.
- **`internal/controller/`** – Reloader reconciler (Config reconciliation, event processing).
- **`internal/listener/`** – Notification source implementations (Kubernetes Secret/ConfigMap, webhook, SQS, etc.).
- **`internal/handler/`** – Destination handlers (ExternalSecret, Deployment, PushSecret, etc.).
- **`Tiltfile`** – Tilt config: kustomize `config/default`, binary build, image build, live update.
