# Additional allowed edge routers

## Intent and approved interface

Allow a hosted `ZitiService` to use named additional routers as service entry or exit points. Keep hosting permission separate from router permission and Dial permission. The operator must resolve exact OpenZiti names to backend IDs. It must not broaden access through role selectors or wildcards.

The user approved the additive design and chose `additionalAllowedEdgeRouterNames`:

```yaml
spec:
  router:
    name: ziti-router
    additionalAllowedEdgeRouterNames:
      - ziti-agibase-router
```

`spec.router.name` keeps its existing meaning: the exact name of the hosting edge router and its Router identity. The optional list names additional edge routers permitted as entry or exit points for this service. The hosting edge router is always included. Omitting the list or setting it to an empty list preserves current behavior.

The existing `ZitiAccessPolicy` interface and the Garage Dial policy remain unchanged.

## Policy behavior

The generated Bind policy includes only the hosting Router identity and the exact service ID. It never includes the additional edge routers. The generated service-edge-router policy includes the exact hosting edge-router ID plus the exact IDs of the listed additional routers, and only this service's ID. Existing policy names, semantics, status IDs, ownership, and cleanup behavior remain unchanged.

The new field grants neither Dial nor Bind permission. It does not create router identities, routers, client access policies, or identity-edge-router policies. It does not alter global policies or other services.

Removing an additional name removes its router from this operator-managed service-edge-router policy on the next successful reconciliation. OpenZiti permissions are additive across policies, so this does not revoke grants from other policies.

## Resolution and failure behavior

Resolve the hosting identity, hosting edge router, and every additional edge router before either router policy is written. Additional routers require an edge-router lookup, not an identity lookup. Use existing exact-name lookup APIs, never interpolate user input into policy role expressions.

A missing router, empty backend ID, or lookup error prevents both router-policy writes for that reconciliation. Report `Ready=False` using the existing failure/status machinery. Retain existing router policies rather than replacing them with a partially resolved router set. Service and config reconciliation can already occur before this point. There is no new transaction guarantee for failures during subsequent OpenZiti write calls.

Implementation choices made by the assistant: trim names consistently with the existing hosting-router behavior, reject blank additional names, model the optional list as a Kubernetes set, and sort/deduplicate resolved edge-router IDs before constructing exact `@ID` roles. Including the hosting name in the additional list must not duplicate its policy role. An empty list is valid. A backend read that lacks an ID is an error, not permission to skip a router.

## Scope and rollout constraint

Implement the API type, generated deepcopy/schema, resolver and policy mapping, automated regression tests, and user documentation. Regenerate the upstream installer through the existing build target. Use the latest fetched operator `origin/main` as the base, not the stale primary checkout. Keep the existing unrelated local release workflow untouched.

Prepare the Garage service change in a separate homelab worktree based on current `origin/main`. Leave the Garage hosting router, host address/port, intercept, role attributes, and Dial policy unchanged.

The homelab ArgoCD application follows `main` with automated sync. Do not push/merge the homelab change to `main`, change the live cluster, rerun probes, or remove probe objects. Do not run Kind/e2e cluster provisioning as part of local verification. A supporting immutable operator image and matching pinned install bundle are prerequisites for any future homelab rollout. Do not invent an image digest or substitute a mutable image tag. If no supporting release exists, leave the existing homelab installer untouched and label the local manifest change as blocked from rollout.

## Acceptance evidence

Automated tests must prove omitted/empty-list compatibility, additional exact router roles, host-only Bind, stable deduplication, name resolution, missing/empty/error backend failures, and absence of router-policy mutations when resolution fails. API-server integration tests must prove the field survives admission/readback, rejects blank names, reports resolution failure, and updates/removes allowed router roles in place without changing service or policy IDs.

Verify generated files through the existing generators, run `go test ./...` with local envtest binaries, run `go vet ./...`, and render the installer locally. Verify homelab YAML with linting/parsing and a focused diff. Do not add tests that only assert generated YAML text, image pins, or documentation.

Baseline on 2026-10-20: `go test ./...` passed at operator commit `d26490622def5a1d563b0e96dabe3af13321b947` with `KUBEBUILDER_ASSETS=/home/roche/projects/miniziti-operator/bin/k8s/1.35.0-linux-amd64`. Homelab base: `982680b7113e92162e1469a19a4304d333040a02`.
