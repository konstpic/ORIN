#!/usr/bin/env bash
# Deploy orin in scaled mode (apiserver + controller + reposerver) via the
# umbrella Helm chart (micro-subcharts). Same image flow as deploy.sh.
#
# Prerequisites: Docker Desktop with Kubernetes enabled, helm, kubectl.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
NS=orin
RELEASE=orin
THIS_DIR="$(cd "$(dirname "$0")" && pwd)"

exec "${THIS_DIR}/deploy.sh" "$@"
