# Quickstart: Miniziti Operator

## Prerequisites

- Go 1.25.5 or newer available in the dev shell
- Kubebuilder v4.10.1 available in the dev shell
- A reachable Kubernetes cluster with CRD v1 support
- Access to an OpenZiti controller URL plus management credentials
- `kubectl`, `make`, and `git`

## 1. Enter the development environment

```bash
nix-shell
go version
kubebuilder version
```

## 2. Scaffold the operator layout

If the repository has not yet been scaffolded, initialize the project with
Kubebuilder using the `sixfeetup.com` domain and the Go plugin, then add the
three APIs:

```bash
kubebuilder init --domain sixfeetup.com --repo example.com/miniziti-operator
kubebuilder create api --group ziti --version v1alpha1 --kind ZitiIdentity --resource --controller
kubebuilder create api --group ziti --version v1alpha1 --kind ZitiService --resource --controller
kubebuilder create api --group ziti --version v1alpha1 --kind ZitiAccessPolicy --resource --controller
```

## 3. Configure operator credentials

Create a namespace for the operator and a Secret containing the only supported
v1 management credential format: controller URL plus username/password:

```bash
kubectl create namespace ziti
kubectl -n ziti create secret generic openziti-management \
  --from-literal=controllerUrl=https://ziti.example.com/edge/management/v1 \
  --from-literal=username=admin \
  --from-literal=password=change-me
```

If the controller uses a private or self-signed management certificate, include
the PEM trust bundle as `caBundle` in the same Secret.

## 4. Implement and validate the CRDs

- Define the schemas in `api/v1alpha1/`
- Generate CRDs and manifests
- Ensure every resource exposes `status.id`, `status.conditions`,
  `status.observedGeneration`, and `status.lastError`

```bash
make generate
make manifests
```

## 5. Run the controller locally

Install CRDs, start the controller, and point it at the credential Secret:

```bash
make install
make run
```

## 6. Apply sample resources

The repository keeps the Kustomize samples in `config/samples/` and the
contract bundle in `specs/001-miniziti-operator/contracts/miniziti-samples.yaml`
in sync. The placeholder Secret manifest in
`config/samples/openziti-management-secret.yaml` is reference-only; do not apply
it over a working cluster Secret. Use either source to exercise the MVP
workflow:

```bash
kubectl apply -k config/samples
kubectl apply -f specs/001-miniziti-operator/contracts/miniziti-samples.yaml
kubectl get zitiidentities,zitiservices,zitiaccesspolicies -A
kubectl describe zitiidentity alice -n default
kubectl describe zitiservice argocd -n default
kubectl describe zitiaccesspolicy argocd-devops-dial -n default
```

In the sample policy, `serviceSelector.matchNames: [argocd]` matches the
OpenZiti service name declared in `ZitiService.spec.name`.

## 7. Run the test suites

```bash
make validate
```

The validation flow runs code generation, manifest generation, formatting,
`go vet`, and the full Go test suite with envtest assets.

Focus coverage on:

- create, update, retry, and delete reconciliation
- finalizer cleanup of operator-owned backend objects
- status and event updates on failures
- enrollment Secret generation when requested

## 8. Review contracts and design artifacts

- API contract: `specs/001-miniziti-operator/contracts/openapi.yaml`
- Sample manifests: `specs/001-miniziti-operator/contracts/miniziti-samples.yaml`
- Data model: `specs/001-miniziti-operator/data-model.md`
- Research decisions: `specs/001-miniziti-operator/research.md`
