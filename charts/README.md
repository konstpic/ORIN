# Helm chart repository

Classic Helm repo (`index.yaml` + `.tgz` artifacts). Regenerate after chart changes:

```bash
make helm-package
```

Install:

```bash
helm repo add orin https://raw.githubusercontent.com/konstpic/ORIN/main/charts
helm search repo orin
helm upgrade --install orin orin/orin --version 0.2.1 -n orin --create-namespace
```

OCI (preferred for releases): `oci://ghcr.io/konstpic/charts/orin`.
