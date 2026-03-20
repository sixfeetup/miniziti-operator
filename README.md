# miniziti-operator

`miniziti-operator` is a scratch-our-own-itch Kubernetes operator for making the
most common OpenZiti actions declarative from cluster manifests. It reconciles
`ZitiIdentity`, `ZitiService`, and `ZitiAccessPolicy` custom resources into the
corresponding OpenZiti identities, services, and access policies.

## Description

The operator treats Kubernetes manifests as the source of truth for a focused
OpenZiti workflow: adding identities, publishing services, and granting user or
workload access to those services through policy declarations. It reads
management credentials from a Kubernetes Secret, creates and updates OpenZiti
objects through the management API, and reports reconciliation status back on
the custom resources with stable status fields such as `status.id`,
`status.conditions`, `status.observedGeneration`, and `status.lastError`.

This project is intentionally narrow. It is not meant to be an exhaustive
implementation of the OpenZiti management API or a complete Kubernetes
integration for every OpenZiti capability. It is aimed at the common declarative
workflow this repository needs most often. If you are looking for the broader
official Kubernetes integration effort around Ziti, see NetFoundry's
[`ziti-k8s-agent`](https://github.com/netfoundry/ziti-k8s-agent).

## Getting Started

### Prerequisites

- go version v1.24.6+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### To Deploy on the cluster

**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/miniziti-operator:tag
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/miniziti-operator:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
> privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

> **NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall

**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Project Distribution

Following the options to release and provide this solution to the users.

### By providing a bundle with all YAML files

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/miniziti-operator:tag
```

**NOTE:** The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without its
dependencies.

2. Using the installer

Users can just run 'kubectl apply -f <URL for YAML BUNDLE>' to install
the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/miniziti-operator/<tag or branch>/dist/install.yaml
```

### By providing a Helm Chart

1. Build the chart using the optional helm plugin

```sh
kubebuilder edit --plugins=helm/v2-alpha
```

2. See that a chart was generated under 'dist/chart', and users
   can obtain this solution from there.

**NOTE:** If you change the project, you need to update the Helm Chart
using the same command above to sync the latest changes. Furthermore,
if you create webhooks, you need to use the above command with
the '--force' flag and manually ensure that any custom configuration
previously added to 'dist/chart/values.yaml' or 'dist/chart/manager/manager.yaml'
is manually re-applied afterwards.

## Contributing

Contributions should keep the operator focused on its declared v1 scope and
preserve the existing controller-runtime and Kubebuilder patterns in the
repository. Before opening a change, run the documented validation flow locally
and keep generated manifests in sync with the code:

```sh
make validate
make test-e2e
```

When changing API types or reconciliation behavior, update the relevant CRDs,
samples, tests, and design artifacts under `specs/001-miniziti-operator/` if the
intended behavior changes. Prefer small, reviewable commits, and avoid
committing local Kustomize mutations or other generated drift that is not part
of the functional change.

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder
Documentation](https://book.kubebuilder.io/introduction.html)

## License

This project is licensed under the Apache License, Version 2.0. See [LICENSE](./LICENSE).
