# End-to-end smoke test

Spins up a kind cluster + an ephemeral Postgres + a local Git repository,
launches `k8s-ui all-in-one`, drives it via REST, and asserts the expected
state transitions.

## Prerequisites

- `kind`, `kubectl`, `docker`, `go`, `git` on `PATH`.
- Port `8080` free for the API server; `5432` for Postgres.

## Run

```bash
cd test/e2e
./run.sh
```

The script:

1. Creates a kind cluster (`k8s-ui-e2e`).
2. Starts Postgres in a container.
3. Initialises a local Git "remote" repo with a single Deployment.
4. Builds and starts the `k8s-ui` binary with the kind kubeconfig.
5. Calls the API to register the repo + create an application, then waits
   for `Synced/Healthy`.
6. Edits the repo to change the replica count, polls Git, and asserts the
   app flips to `OutOfSync`.
7. Triggers a sync, waits for `Synced/Healthy` again.
8. Tears everything down on exit.
