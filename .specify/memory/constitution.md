<!--
Sync Impact Report
- Version change: 1.0.0 -> 1.1.0
- Modified principles:
  - I. Test-First Delivery (TDD) -> I. Kubernetes API Contracts Are Stable
  - II. Tidy First (Separate Structural vs Behavioral) -> II. Reconciliation Is Idempotent and Safe
  - III. Simplicity Over Speculation -> V. Operator Scope Stays Narrow
  - IV. Quality Gates Are Non-Negotiable -> IV. Controller Behavior Is Proven by Tests
- Added sections: Operator Scope & Platform Constraints, Delivery Workflow & Quality Gates
- Removed sections: None
- Templates requiring updates:
  - ✅ .specify/templates/plan-template.md
  - ✅ .specify/templates/spec-template.md
  - ✅ .specify/templates/tasks-template.md
  - ⚠ .specify/templates/commands/ (directory missing; no files to update)
- Follow-up TODOs:
  - TODO(RATIFICATION_DATE): original ratification date not found in repo
-->

# Constitution

## Core Principles

### I. Kubernetes API Contracts Are Stable

The `ziti.sixfeetup.com` CRDs and their status fields are external contracts.
Every change to `spec`, `status`, defaults, validation, or resource names MUST
preserve upgrade safety for existing manifests or include an explicit migration
plan approved in the feature spec. `status.observedGeneration`,
machine-readable conditions, and provider object identifiers MUST remain
deterministic and documented. Rationale: operator users depend on CRDs as an
API, not just an internal struct.

### II. Reconciliation Is Idempotent and Safe

Controllers MUST converge from repeated reconcile loops without creating
duplicate OpenZiti objects, leaking credentials, or depending on single-shot
execution. Each reconcile path MUST tolerate retries, partial failures, and
out-of-order events. Finalizers MUST clean up only resources this operator owns
unless a retention policy is explicitly designed and documented. Rationale:
Kubernetes controllers are retried continuously and unsafe side effects become
production incidents.

### III. Status, Events, and Errors Explain Reality

Every managed resource MUST publish enough status to explain current state
without external debugging: OpenZiti object IDs, readiness conditions, observed
generation, and actionable failure messages. Controllers MUST emit Kubernetes
events for create, update, and reconcile failures, and MUST never leave status
stale after a completed reconcile attempt. Rationale: operators are operable
only when the cluster itself exposes what is happening.

### IV. Controller Behavior Is Proven by Tests

Every behavioral change MUST begin with a failing automated test. Changes to
CRD schemas, selectors, finalizers, status handling, secret generation, or API
interactions MUST include targeted controller or reconciliation tests that
prove success, retry, and deletion behavior. The relevant automated suites MUST
pass before merge, with `go test ./...` as the minimum project-wide gate once
the Go module exists. Rationale: controller defects usually appear in edge
cases and retries, not in happy-path manual checks.

### V. Operator Scope Stays Narrow

The MVP manages only `ZitiIdentity`, `ZitiService`, and `ZitiAccessPolicy`
against the OpenZiti Edge Management API. New CRDs, broader selector
languages, router-policy management, packaging systems, or speculative
abstractions MUST not be introduced without a concrete requirement captured in
spec and plan artifacts. Prefer the smallest implementation that preserves
clear ownership of identities, services, policies, and enrollment secrets.
Rationale: a small operator surface is easier to reason about, test, and
operate.

## Operator Scope & Platform Constraints

- Kubernetes manifests MUST be the source of truth for the OpenZiti objects the
  operator manages.
- The implementation target is Go with `controller-runtime`; deviations MUST be
  justified in the implementation plan.
- Managed resources MUST reconcile against the OpenZiti Edge Management API
  using deterministic lookup keys and stored status IDs.
- Secrets and credentials MUST be sourced from Kubernetes Secrets or equivalent
  cluster configuration, never hard-coded in manifests or tests.
- Enrollment JWT secrets MUST only be created or refreshed when the resource
  spec explicitly enables that behavior.
- The MVP boundary excludes router-policy CRDs, service-edge-router policy
  CRDs, posture checks, and full selector-language parity unless this
  constitution is amended.

## Delivery Workflow & Quality Gates

- Specs MUST describe CRD schema impact, reconciliation invariants,
  status/event changes, deletion behavior, and any secret or RBAC implications.
- Plans MUST pass a constitution check covering API compatibility, idempotent
  reconciliation, observability, test strategy, and MVP-scope discipline before
  implementation starts.
- Tasks MUST separate structural work from behavioral work and include explicit
  coverage for tests, status conditions, events, finalizers, and documentation
  when those surfaces change.
- Code review MUST reject changes that mutate external API shape, cleanup
  semantics, or OpenZiti side effects without corresponding tests and rollout
  notes.
- Before merge, contributors MUST run the relevant automated suites and
  document any intentionally deferred work.

## Governance

- This constitution supersedes local conventions for operator design, testing,
  and review.
- Amendments require a documented rationale, synchronized template updates, and
  review of downstream workflow impacts.
- Versioning follows SemVer: MAJOR for principle removals/redefinitions,
  MINOR for new principles/sections or materially expanded guidance, PATCH for
  clarifications and wording fixes.
- Compliance is checked in feature specs, implementation plans, task lists, and
  pull requests.
- Runtime guidance files such as `AGENTS.md`, if introduced, MUST remain
  consistent with this constitution.

**Version**: 1.1.0 | **Ratified**: TODO(RATIFICATION_DATE): original ratification date not found in repo | **Last Amended**: 2026-03-14
