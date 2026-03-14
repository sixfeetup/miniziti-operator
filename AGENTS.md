# miniziti-operator Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-03-14

## Active Technologies
- Go 1.25.5 + Kubebuilder v4.10.1 scaffolding, controller-runtime, controller-tools/kustomize, generated OpenZiti Edge Management API client (001-miniziti-operator)
- Kubernetes API objects and status, Kubernetes Secrets, OpenZiti Edge Management API (001-miniziti-operator)

- Go 1.24
- Kubebuilder `go/v4` scaffold
- `controller-runtime`
- Kubernetes API objects and status
- Kubernetes Secrets
- OpenZiti `edge-api`
- `controller-gen`
- `setup-envtest`

## Project Structure

```text
api/
cmd/
config/
internal/
test/
specs/
```

## Commands

- `go test ./...`
- `controller-gen crd rbac paths=./... output:crd:artifacts:config=config/crd/bases`
- `setup-envtest use --bin-dir ./bin/k8s <k8s-version>`

## Code Style

Go 1.24: Follow standard Go conventions and controller-runtime reconciliation
patterns.

## Recent Changes
- 001-miniziti-operator: Added Go 1.25.5 + Kubebuilder v4.10.1 scaffolding, controller-runtime, controller-tools/kustomize, generated OpenZiti Edge Management API client

- `001-miniziti-operator`: Added the initial Miniziti operator planning artifacts
  and Go/Kubebuilder/controller-runtime stack selection.

<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->
