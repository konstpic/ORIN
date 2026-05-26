# Helm chart repository

Classic Helm repo (`index.yaml` + `.tgz` artifacts). Regenerate after chart changes:

```bash
make helm-package
```

Install (images pull from **Docker Hub** by default):

```bash
helm repo add orin https://raw.githubusercontent.com/konstpic/ORIN/main/charts
helm search repo orin
helm upgrade --install orin orin/orin --version 0.2.3 -n orin --create-namespace
```

Images: `konstpic/orin-apiserver`, `konstpic/orin-controller`, `konstpic/orin-reposerver` (tag `0.2.2` unless overridden).
