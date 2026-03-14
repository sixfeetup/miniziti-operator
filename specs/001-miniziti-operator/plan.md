# Implementation Plan: Miniziti Operator

**Branch**: `001-miniziti-operator` | **Date**: 2026-03-14 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/001-miniziti-operator/spec.md`

## Summary

Build a small Kubernetes operator named `miniziti` that manages three CRDs,
`ZitiIdentity`, `ZitiService`, and `ZitiAccessPolicy`, as the cluster-facing
source of truth for selected OpenZiti objects. The implementation will use a
standard Kubebuilder/controller-runtime layout, reconcile to the OpenZiti Edge
Management API through a typed internal client adapter, and keep the MVP narrow
to identities, services, access policies, and optional enrollment JWT secrets.

## Technical Context

**Language/Version**: Go 1.25.5  
**Primary Dependencies**: Kubebuilder v4.10.1 scaffolding, controller-runtime, controller-tools/kustomize, generated OpenZiti Edge Management API client  
**Storage**: Kubernetes API objects and status, Kubernetes Secrets, OpenZiti Edge Management API  
**Testing**: `go test ./...`, envtest controller tests, fake OpenZiti client unit tests, focused e2e smoke tests  
**Target Platform**: Kubernetes clusters with `apiextensions.k8s.io/v1` CRD support; local scaffolding assets from Kubebuilder Kubernetes 1.34.1  
**Project Type**: Kubernetes operator  
**API Surface**: `ziti.sixfeetup.com/v1alpha1` CRDs for `ZitiIdentity`, `ZitiService`, and `ZitiAccessPolicy`; `ZitiIdentity.spec.type` enum values `User|Device|Service` mapped from employee, node, and workload declarations respectively; explicit `intercept` and `host` blocks in `ZitiService`; status fields `id`, `conditions`, `observedGeneration`, `lastError`; finalizers; RBAC; events  
**Performance Goals**: Valid resources reach `Ready` or an actionable failure state within 60 seconds in a single-cluster test environment with a reachable OpenZiti controller, no injected failures, and fewer than 100 managed resources under reconciliation; duplicate-free convergence across 10 repeated applies; deleting a single managed declaration with a reachable backend and no concurrent controller restart completes cleanup within 2 minutes  
**Constraints**: Idempotent reconciliation, deterministic external naming, namespaced credentials from Secrets, upgrade-safe v1alpha1 schemas, generated OpenZiti client wrapped behind an internal adapter, no raw arbitrary service-config payloads, no router-policy CRDs, no Operator SDK/OLM packaging, no full OpenZiti parity in v1  
**Scale/Scope**: MVP targets a single cluster deployment managing low hundreds of identities, services, and access policies across one or a small set of namespaces  
**External Systems**: OpenZiti Edge Management API, Kubernetes Secret storage, Kubernetes admission/defaulting for CRDs

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- API contract: PASS. The design fixes the external API to three
  `ziti.sixfeetup.com/v1alpha1` CRDs with stable status fields and no schema
  expansion beyond the MVP resource set.
- Reconciliation safety: PASS. Reconcilers will use stored external IDs plus
  deterministic names derived from namespace and resource name, with finalizers
  limited to deleting operator-owned OpenZiti objects and generated secrets.
- Observability: PASS. Every CRD includes `status.id`, `status.conditions`,
  `status.observedGeneration`, and `status.lastError`, plus Kubernetes events
  for create, update, and failure paths.
- Test-first delivery: PASS. Controller behavior will be covered by envtest and
  fake-client tests for create, retry, update, and delete flows before merging.
- Scope discipline: PASS. MVP scope remains limited to identities, services,
  access policies, and enrollment JWT output; router policies, richer selector
  languages, and packaging layers stay out of scope.

## Project Structure

### Documentation (this feature)

```text
specs/001-miniziti-operator/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── miniziti-samples.yaml
│   └── openapi.yaml
└── tasks.md
```

### Source Code (repository root)

```text
.gitignore
Makefile
api/
└── v1alpha1/
cmd/
└── main.go
config/
├── crd/
├── default/
├── manager/
├── rbac/
└── samples/
internal/
├── controller/
├── credentials/
└── openziti/
    ├── client/
    ├── identity/
    ├── policy/
    └── service/
hack/
test/
├── integration/
└── e2e/
```

**Structure Decision**: Use the standard Kubebuilder layout for API types,
controller entrypoints, manifests, and tests. Add a dedicated
`internal/openziti/` adapter layer so reconciliation logic depends on a typed
interface rather than raw HTTP calls, and keep secret/config handling isolated
under `internal/credentials/`.

## Post-Design Constitution Check

- API contract: PASS. `contracts/openapi.yaml` documents the Kubernetes-facing
  resource paths and schemas for the three CRDs, and `data-model.md` defines
  stable status surfaces for v1alpha1.
- Reconciliation safety: PASS. The design keeps explicit external ID tracking,
  deterministic names, bounded selector semantics, and finalizer ownership
  rules in the data model and quickstart flow.
- Observability: PASS. The data model requires actionable status and condition
  updates for each resource lifecycle step, and quickstart validation includes
  checking status and events.
- Test-first delivery: PASS. Research and quickstart both anchor the test
  strategy in envtest and fake-client coverage before implementation.
- Scope discipline: PASS. Contracts and data model only cover `ZitiIdentity`,
  `ZitiService`, `ZitiAccessPolicy`, and optional enrollment JWT secret output.

## Complexity Tracking

No constitution violations currently require justification.
