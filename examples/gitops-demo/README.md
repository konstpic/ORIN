# gitops-demo — примеры деплоя для ORIN

Этот каталог — готовый **демо GitOps репозиторий** внутри репозитория ORIN. Он покрывает два сценария:

1. **Одиночные приложения** (каждое приложение указывает на свой `path` в Git).
2. **App-of-apps**: одно “родительское” приложение указывает на Helm chart, который рендерит декларации дочерних `Application`/`AppProject`.

ORIN распознаёт `orin.dev/v1alpha1` `Application` и `AppProject` в отрендеренных манифестах и создаёт/обновляет соответствующие записи в базе. Эти control-plane объекты **не применяются** в целевой кластер.

## Одиночные приложения

### Вариант A: plain YAML

- repoURL: `https://github.com/konstpic/ORIN.git`
- path: `examples/gitops-demo/kubernetes`
- targetRevision: `main`

### Вариант B: Helm chart

- repoURL: `https://github.com/konstpic/ORIN.git`
- path: `examples/gitops-demo/samples/hello-world`
- targetRevision: `main`

## App-of-apps (bootstrap)

### Шаг 1 — зарегистрировать репозиторий

```
URL: https://github.com/konstpic/ORIN.git
```

### Шаг 2 — создать “родительское” приложение

```bash
curl -X POST http://localhost:8080/api/v1/applications \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "gitops-demo-stack",
    "project": "default",
    "source": {
      "repoUrl": "https://github.com/konstpic/ORIN.git",
      "path": "examples/gitops-demo/deploy/.helm",
      "targetRevision": "main"
    },
    "destination": { "cluster": "in-cluster", "namespace": "gitops-demo-stack" },
    "syncPolicy": {
      "automated": { "prune": true, "selfHeal": true },
      "createNamespace": true
    }
  }'
```

После первого reconcile ORIN:

- применит “прикладные” объекты дочерних приложений (Namespace/ServiceAccount и т.п., если они есть);
- подхватит `orin.dev/v1alpha1 AppProject` и `orin.dev/v1alpha1 Application` из рендера и создаст дочерние приложения в ORIN.

## Apps catalog (опционально)

Если хочешь, чтобы ORIN сам создавал bootstrap-приложение по YAML файлу, включи apps catalog и укажи файл:

- repoUrl: `https://github.com/konstpic/ORIN.git`
- path: `examples/gitops-demo/orin/apps.yaml`

## Структура каталога

| Path | Тип | Назначение |
|------|-----|------------|
| `deploy/.helm` | Helm (родительский chart) | Рендерит `AppProject`/`Application` для дочерних приложений |
| `kubernetes/` | Plain YAML | Набор манифестов для “web” |
| `samples/hello-world/` | Helm chart | Демо chart для “hello-world” |
| `orin/apps.yaml` | Apps catalog | Описание bootstrap-приложения для controller poll |
