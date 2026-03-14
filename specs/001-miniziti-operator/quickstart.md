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

Create a namespace for the operator and a Secret containing the OpenZiti
controller URL and management credentials:

```bash
kubectl create namespace miniziti-system
kubectl -n miniziti-system create secret generic openziti-management \
  --from-literal=controllerUrl=https://ziti.example.com/edge/management/v1 \
  --from-literal=username=admin \
  --from-literal=password=change-me
```

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

Create one identity, one service, and one access policy to exercise the MVP
workflow:

```yaml
apiVersion: ziti.sixfeetup.com/v1alpha1
kind: ZitiIdentity
metadata:
  name: alice
  namespace: default
spec:
  name: alice@example.com
  type: User
  roleAttributes:
    - employee
    - devops
  enrollment:
    createJwtSecret: true
    jwtSecretName: alice-ziti-jwt
---
apiVersion: ziti.sixfeetup.com/v1alpha1
kind: ZitiService
metadata:
  name: argocd
  namespace: default
spec:
  name: argocd
  roleAttributes:
    - argocd
  configs:
    intercept:
      protocols:
        - tcp
      addresses:
        - argocd.ziti
      portRanges:
        - low: 443
          high: 443
    host:
      protocol: tcp
      address: argocd-server.argocd.svc.cluster.local
      port: 443
---
apiVersion: ziti.sixfeetup.com/v1alpha1
kind: ZitiAccessPolicy
metadata:
  name: argocd-devops-dial
  namespace: default
spec:
  type: Dial
  identitySelector:
    matchRoleAttributes:
      - devops
  serviceSelector:
    matchNames:
      - argocd
```

Apply the sample manifest and inspect readiness:

```bash
kubectl apply -f specs/001-miniziti-operator/contracts/miniziti-samples.yaml
kubectl get zitiidentities,zitiservices,zitiaccesspolicies -A
kubectl describe zitiidentity alice
```

## 7. Run the test suites

```bash
go test ./...
```

Focus test coverage on:

- create, update, retry, and delete reconciliation
- finalizer cleanup of operator-owned backend objects
- status and event updates on failures
- enrollment Secret generation when requested

## 8. Review contracts and design artifacts

- API contract: `specs/001-miniziti-operator/contracts/openapi.yaml`
- Sample manifests: `specs/001-miniziti-operator/contracts/miniziti-samples.yaml`
- Data model: `specs/001-miniziti-operator/data-model.md`
- Research decisions: `specs/001-miniziti-operator/research.md`
