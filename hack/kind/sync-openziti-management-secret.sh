#!/usr/bin/env bash
set -euo pipefail

OPENZITI_NAMESPACE="${OPENZITI_NAMESPACE:-ziti}"
OPENZITI_RELEASE="${OPENZITI_RELEASE:-ziti-controller}"
OPENZITI_ADMIN_SECRET="${OPENZITI_ADMIN_SECRET:-${OPENZITI_RELEASE}-admin-secret}"
OPENZITI_MGMT_TLS_SECRET="${OPENZITI_MGMT_TLS_SECRET:-${OPENZITI_RELEASE}-web-mgmt-api-secret}"
OPERATOR_NAMESPACE="${OPERATOR_NAMESPACE:-ziti}"
MANAGEMENT_SECRET_NAME="${MANAGEMENT_SECRET_NAME:-openziti-management}"
OPENZITI_MGMT_URL="${OPENZITI_MGMT_URL:-https://${OPENZITI_RELEASE}-mgmt.${OPENZITI_NAMESPACE}.svc.cluster.local/edge/management/v1}"

username="$(
  kubectl get secret "${OPENZITI_ADMIN_SECRET}" \
    -n "${OPENZITI_NAMESPACE}" \
    -o go-template='{{if index .data "admin-username"}}{{index .data "admin-username" | base64decode}}{{else}}admin{{end}}'
)"

password="$(
  kubectl get secret "${OPENZITI_ADMIN_SECRET}" \
    -n "${OPENZITI_NAMESPACE}" \
    -o go-template='{{index .data "admin-password" | base64decode}}'
)"

ca_bundle="$(
  kubectl get secret "${OPENZITI_MGMT_TLS_SECRET}" \
    -n "${OPENZITI_NAMESPACE}" \
    -o go-template='{{index .data "ca.crt" | base64decode}}'
)"

kubectl create namespace "${OPERATOR_NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

kubectl create secret generic "${MANAGEMENT_SECRET_NAME}" \
  --namespace "${OPERATOR_NAMESPACE}" \
  --from-literal=controllerUrl="${OPENZITI_MGMT_URL}" \
  --from-literal=username="${username}" \
  --from-literal=password="${password}" \
  --from-literal=caBundle="${ca_bundle}" \
  --dry-run=client \
  -o yaml | kubectl apply -f -
