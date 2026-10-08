# miniziti-operator

`miniziti-operator` is a scratch-our-own-itch Kubernetes operator for making
the most common OpenZiti actions declarative from cluster manifests. It
reconciles `ZitiIdentity`, `ZitiService`, and `ZitiAccessPolicy` custom
resources into the corresponding OpenZiti identities, services, service hosting
policies, and access policies. `ZitiEntraRoleSync` keeps attributes of enrolled
identities in line with Entra group app-role assignments.

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
  password: "[REDACTED]"
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

Example service with router-side hosting enabled:

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
  router:
    name: ziti-prod-router
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

When `spec.router.name` is set, the operator also creates the OpenZiti `Bind`
service policy and service-edge-router policy needed for the named router to
host the service. The router name must match an existing OpenZiti edge router
and its router identity.

Dial access remains a separate `ZitiAccessPolicy`, so you can grant client/user
access independently from service hosting.

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
- `status.configIDs` on `ZitiService`
- `status.bindPolicyID` and `status.serviceEdgeRouterPolicyID` on routed `ZitiService` resources

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

## Entra group role synchronization

An app role Value is the string that an Entra role grants. A managed attribute
is an OpenZiti role attribute that this sync controls. The sync adds and removes
these attributes on existing identities. It never creates identities or changes
their names, external IDs, administrator permissions, tags, or authentication
policies. Direct user assignments and nested groups do not grant attributes.

The sync enforces access from its first run. There is no report-only mode.
Read the first-sync procedure below before creating a resource.

### Directory reader and identity matching

Create a separate Entra app registration to read the directory. Give it these
Microsoft Graph application permissions, with administrator consent for each:

- `Application.Read.All`
- `GroupMember.ReadBasic.All`
- `User.ReadBasic.All`

The resource's `appId` identifies the app whose roles control Ziti access. It
is not the directory reader's client ID. The resource's `tenantId` identifies
the Entra tenant. The operator supports only the public Microsoft cloud.

Set `userProperty` to match the external JWT signer's `claimsProperty`.
For an `email` claim, use `mail`. For an `oid` claim, use `id`.
The third supported property is `userPrincipalName`. Matching with the Ziti
identity's `externalId` ignores case. The defaults are `mail` and a `10m`
interval. The minimum interval is `1m`.

Identities without an external ID are unchanged. If identities share the same
external ID, the sync skips all of them. This includes duplicates held by other
controllers. A `ZitiIdentity` resource owns its identity's complete attribute
list. The Entra sync excludes that identity by recorded ID or exact adoption
name, across all namespaces. Choose one resource kind to control an identity.

### Credentials and resource

Create the reader's Secret in the same namespace as the sync. Replace the
placeholder values before applying this manifest. Do not commit credentials.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: ziti-role-sync-entra
  namespace: ziti
type: Opaque
stringData:
  clientId: replace-with-directory-reader-client-id
  clientSecret: replace-with-directory-reader-client-secret
```

The Ziti connection still uses the operator's existing `openziti-management`
Secret. The Entra Secret contains no Ziti configuration. The operator reads it
again on each scheduled sync. Secret edits do not trigger an immediate sync.

Copy [the sample](config/samples/ziti_v1alpha1_zitientrarolesync.yaml).
Replace `tenantId` and `appId` with your GUIDs. Set `userProperty` for your
signer. Do not apply the resource until you complete the snapshot step below.

### First-sync procedure

Use an authenticated OpenZiti management client to export every identity before
you create the resource. Include `id`, `name`, `externalId`, and
`roleAttributes`. Follow every result page. Save the complete response as
`identities-before.json`. A first-page-only export cannot show all changes.

1. List the app's role Values and the attributes that you expect to add or remove.
2. Save `identities-before.json` outside the cluster.
3. Apply the reader Secret.
4. Apply your edited `ZitiEntraRoleSync` manifest.
5. Read the resource status with the commands below.
6. Export every identity again as `identities-after.json`.
7. Compare identities by ID and compare attribute sets, not list order.
8. Make sure that every grant and removal matches the app's group assignments.
9. Make sure that unmanaged attributes and excluded identities remain unchanged.

```sh
kubectl get zers -A
kubectl describe zitientrarolesync example -n ziti
kubectl get zitientrarolesync example -n ziti -o yaml
```

The operator reads roles, assignments, group members, identities, and ownership
claims before any Ziti write. It reads each identity and its ownership again
before its PATCH. An external edit after that fresh read can still race the
PATCH. Updates across identities are not atomic.

### Status and cleanup

A `Synced` Ready condition means that eligible writes and retirement cleanup
succeeded. `lastSyncTime` advances only then. The write-phase counters show
matched identities, successful updates, and granted users without identities.
The unmatched report contains at most 50 entries. This limit does not reduce
cleanup tracking.

Disabled roles remain managed but grant nobody. A retired Value is a role Value
that disappeared from the app. The operator records new claims in
`status.managedAttributes` before writing attributes. It retains retired
claims until cleanup succeeds, including after a partial PATCH failure.

If a skipped identity still holds a retired Value, the resource reports
`CleanupPending`. Its last successful sync time stays unchanged. Resolve
duplicate external IDs to let the sync clean up those identities. For an
excluded identity, remove the retired Value through its owning controller or
by hand. A `ZitiIdentity` controller restores its declared attributes, so edit
that resource's `spec.roleAttributes` instead of only changing the backend.
Do not clear retained claims from status.

If another sync owns a Value, the resource reports `Conflict`. One recorded
claimant owns the Value even if an older unrecorded contender exists. If
recorded claims overlap, the oldest resource wins. Equal creation times use
namespace, then name, in lexical order. The winner can continue despite the
loser's overlapping claim.

Missing Secrets or keys report `InvalidSpec`. Token and Graph failures report
`GraphError`. A permission failure or incomplete read never authorizes a Ziti
write. A `ZitiError` after earlier PATCHes leaves those successful changes in
place and retains all claims for recovery.

If a plan removes all managed attributes from every eligible holder, the
resource reports `WouldRemoveAll` and writes nothing. There is no override.
Make sure that matching and group assignments are correct. To remove all
managed attributes on purpose, delete the resource and remove them by hand.

Deleting `ZitiEntraRoleSync` stops synchronization without cleanup. There is
no finalizer. Existing identity attributes remain in place. Remove any unwanted
attributes separately before relying on deletion to revoke access.

## Helm

Helm packaging is the next documentation step. For now, the supported
user-facing install path in this repository is the `kubectl apply` flow above.

## Development

Development and contributor-focused instructions live in
[DEVELOPMENT.md](./DEVELOPMENT.md).

## License

This project is licensed under the Apache License, Version 2.0. See
[LICENSE](./LICENSE).
