#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
RESULTS_DIR="${SCRIPT_DIR}/results/$(date +%Y%m%d-%H%M%S)"

echo "==> Creating Kind cluster"
kind create cluster --config "${SCRIPT_DIR}/kind-benchmark.yaml"

echo "==> Installing GIE CRDs"
kubectl apply -f "${REPO_ROOT}/install/helm/agentgateway-crds/templates/"

echo "==> Installing agentgateway"
helm install agentgateway "${REPO_ROOT}/install/helm/agentgateway" \
  --namespace benchmark --create-namespace \
  --set inferenceExtension.enabled=true

echo "==> Deploying test stack"
# kubectl apply -f "${REPO_ROOT}/test/e2e/features/agentgateway/inferenceextension/testdata/"
kubectl apply -f "${REPO_ROOT}/test/e2e/features/inferenceextension/testdata/"

echo "==> Waiting for pods"
kubectl rollout status deployment/llm-sim -n benchmark
kubectl rollout status deployment/epp      -n benchmark

mkdir -p "${RESULTS_DIR}"

echo "==> Running Stack A (baseline: plain k8s Service)"
go run github.com/kubernetes-sigs/inference-perf/cmd/inference-perf \
  --target   http://llm-sim-svc.benchmark.svc.cluster.local:8080 \
  --model    llama-3 \
  --concurrency 10 \
  --duration 60s \
  --output   "${RESULTS_DIR}/baseline.json"

echo "==> Running Stack B (agentgateway + EPP)"
go run github.com/kubernetes-sigs/inference-perf/cmd/inference-perf \
  --target   http://agentgateway.benchmark.svc.cluster.local:8080 \
  --model    llama-3 \
  --concurrency 10 \
  --duration 60s \
  --output   "${RESULTS_DIR}/agentgateway.json"

echo "==> Comparing results"
go run "${SCRIPT_DIR}/compare/main.go" \
  --baseline "${RESULTS_DIR}/baseline.json" \
  --agw      "${RESULTS_DIR}/agentgateway.json"

echo "==> Done. Results in ${RESULTS_DIR}"