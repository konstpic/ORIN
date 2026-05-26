# Scaled manifests (reference)

Production installs should use the **umbrella Helm chart** at
[`../helm/`](../helm/) — subcharts for `common`, `postgres`, `apiserver`,
`controller`, and `reposerver`.

The YAML files in this directory (`01-apiserver.yaml`, etc.) are kept as a
readable reference and for environments without Helm. They may drift from the
chart; treat `deploy/helm/charts/*` as the source of truth.

```bash
./deploy/docker-desktop/deploy.sh
# or (alias)
./deploy/docker-desktop/deploy-scaled.sh
```
