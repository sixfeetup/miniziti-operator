# miniziti-operator

`miniziti-operator` is a scratch-our-own-itch Kubernetes operator for making
the most common OpenZiti actions declarative from cluster manifests. It
reconciles `ZitiIdentity`, `ZitiService`, and `ZitiAccessPolicy` custom
resources into the corresponding OpenZiti identities, services, and access
policies.

The operator is intentionally narrow. It focuses on the common declarative
workflow of:

- adding identities
- publishing services
- granting access to those services

It is not intended to be an exhaustive implementation of the OpenZiti
management API. For the broader official Kubernetes integration effort around
Ziti, see NetFoundry's
[`ziti-k8s-agent`](https://github.com/netfoundry/ziti-k8s-agent).

## Install With kubectl

### Prerequisites

- a Kubernetes cluster you can access with `kubectl`
- an OpenZiti controller URL and management credentials
- optionally, a PEM CA bundle if your OpenZiti management endpoint uses a
  private or self-signed certificate

### 1. Install the operator

The repository publishes an install bundle that uses the Docker Hub image
`sixfeetup/miniziti-operator:latest`:

```sh
kubectl apply -f https://raw.githubusercontent.com/sixfeetup/miniziti-operator/main/dist/install.yaml
```

This installs the CRDs, RBAC, namespace, and controller deployment.

### 2. Create the OpenZiti management Secret

The operator reads OpenZiti management credentials from a Kubernetes Secret
named `openziti-management` in the `ziti` namespace:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: openziti-management
  namespace: ziti
type: Opaque
stringData:
  controllerUrl: https://ziti.example.com/edge/management/v1
  username: admin
  password: change-me
```

If your management endpoint uses a private or self-signed CA, add `caBundle` to
the same Secret:

```yaml
  caBundle: |
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
```

Apply the Secret:

```sh
kubectl apply -f openziti-management-secret.yaml
```

### 3. Create resources

Apply declarative resources for identities, services, and access policies.

Example identity:

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
```

Example service:

```yaml
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
```

Example access policy:

```yaml
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

Apply your manifests:

```sh
kubectl apply -f your-resources.yaml
```

### 4. Check status

```sh
kubectl get zitiidentities,zitiservices,zitiaccesspolicies -A
kubectl describe zitiidentity alice -n default
kubectl describe zitiservice argocd -n default
kubectl describe zitiaccesspolicy argocd-devops-dial -n default
```

The operator reports reconciliation state through status fields including:

- `status.id`
- `status.conditions`
- `status.observedGeneration`
- `status.lastError`

### 5. Uninstall

Delete your custom resources first if you want the operator to reconcile their
removal before uninstall:

```sh
kubectl delete -f your-resources.yaml
```

Then remove the operator bundle:

```sh
kubectl delete -f https://raw.githubusercontent.com/sixfeetup/miniziti-operator/main/dist/install.yaml
```

## Helm

Helm packaging is the next documentation step. For now, the supported
user-facing install path in this repository is the `kubectl apply` flow above.

## Development

Development and contributor-focused instructions live in
[DEVELOPMENT.md](./DEVELOPMENT.md).

## License

This project is licensed under the Apache License, Version 2.0. See
[LICENSE](./LICENSE).
