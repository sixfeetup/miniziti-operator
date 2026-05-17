# ZitiService Router Policy Design

## Context

The existing `ZitiService` CRD reconciles the OpenZiti intercept config, host config, and service. A service created that way is not fully usable for hosted traffic because OpenZiti also needs:

- a `Bind` service policy that lets the hosting router bind the service; and
- a service-edge-router policy that lets the service use that router.

The legacy `../siyavula.deploy/scripts/add-ziti-service.sh` creates both router-side objects for each service.

## Design

Extend `ZitiService` with an optional `spec.router.name` field. The field is the exact existing OpenZiti router/hosting identity name, for example `ziti-prod-router`.

When `spec.router.name` is set, the `ZitiService` reconciler also owns two deterministic OpenZiti objects:

- `<service>-bind-policy`: a `Bind` service policy with `serviceRoles=["@<service>"]` and `identityRoles=["@<router>"]`.
- `<service>-only`: a service-edge-router policy with `serviceRoles=["@<service>"]` and `edgeRouterRoles=["@<router>"]`.

Dial permissions remain separate `ZitiAccessPolicy` resources because they express client/user authorization rather than service hosting infrastructure.

## Scope

This design intentionally does not add first-class router CRDs, generic router selectors, cluster enums, or multi-router support. Those can be added later if real use cases require them.

## Status

`ZitiServiceStatus` records the managed bind policy ID and service-edge-router policy ID when router wiring is enabled.
