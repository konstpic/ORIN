# ORIN CRDs

Kubernetes API extensions for the ORIN app-of-apps pattern:

| Kind | API | Purpose |
|------|-----|---------|
| `Application` | `orin.dev/v1alpha1` | Declares a GitOps application (child of a parent chart). |
| `AppProject` | `orin.dev/v1alpha1` | Declares project policy scope for child apps. |

ORIN reads these objects from rendered Helm manifests and upserts rows in its database.
They are **not** applied as workloads to the destination cluster during sync (the controller filters them out).

## Install

```bash
kubectl apply -f deploy/crds/
# or
kubectl apply -k deploy/crds/
```

Verify:

```bash
kubectl get crd applications.orin.dev appprojects.orin.dev
kubectl api-resources | grep orin.dev
```

## Legacy API groups

Charts may still use `orin.io/v1alpha1` or `k8s-ui.io/v1alpha1` — ORIN recognises them for catalog parsing.
Prefer **`orin.dev/v1alpha1`** for new Git repos.
