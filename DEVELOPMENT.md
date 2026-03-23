# Development

## Development environment

- Go 1.25.5 or newer
- `kubectl`
- Docker
- `make`
- Access to a Kubernetes cluster for manual testing

For local validation and end-to-end testing, the repository also uses Kind,
Helm, and envtest binaries managed through the Makefile.

## Common commands

```sh
make help
make validate
make test
make test-e2e
make lint
```

`make validate` is the main local validation flow. It runs generation,
manifest updates, formatting, `go vet`, and the Go test suite with envtest
assets.

## Local image and deploy flow

Build and push an image to a registry you can access from your cluster:

```sh
make docker-build docker-push IMG=<registry>/miniziti-operator:<tag>
```

Deploy the operator with that image:

```sh
make install
make deploy IMG=<registry>/miniziti-operator:<tag>
```

Generate a single install bundle using a specific image:

```sh
make build-installer IMG=<registry>/miniziti-operator:<tag>
```

## Kind and e2e workflow

The repository includes a Kind-based e2e workflow that installs a real
OpenZiti controller into the cluster before exercising the operator.

Useful commands:

```sh
make setup-test-e2e
make kind-openziti-install
make kind-openziti-sync-management-secret OPERATOR_NAMESPACE=ziti
make test-e2e
make cleanup-test-e2e
```

## Project scope

Keep changes aligned with the current operator scope:

- `ZitiIdentity`
- `ZitiService`
- `ZitiAccessPolicy`
- related enrollment Secret handling

This repository is intentionally not a full implementation of the OpenZiti
management API. Changes should support the common declarative workflow rather
than expand toward exhaustive OpenZiti object coverage unless the project scope
is intentionally changed.

## Contributing

Before opening a change:

```sh
make validate
```

For controller, API, or reconciliation changes:

- update tests
- keep generated manifests in sync
- update docs and samples when behavior changes
- avoid committing local Kustomize drift or unrelated generated changes

Relevant design artifacts live under `specs/001-miniziti-operator/`.
