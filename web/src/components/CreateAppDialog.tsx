import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "../api/client";

export function CreateAppDialog({ onClose }: { onClose: () => void }) {
  const qc = useQueryClient();
  const { data: repos } = useQuery({ queryKey: ["repos"], queryFn: api.listRepos });
  const { data: clusters } = useQuery({ queryKey: ["clusters"], queryFn: api.listClusters });
  const [name, setName] = useState("");
  const [repoUrl, setRepoUrl] = useState("");
  const [path, setPath] = useState(".");
  const [targetRevision, setTargetRevision] = useState("HEAD");
  const [cluster, setCluster] = useState("in-cluster");
  const [namespace, setNamespace] = useState("default");
  const [autoSync, setAutoSync] = useState(false);
  const [createNamespace, setCreateNamespace] = useState(false);
  const [syncOptionsText, setSyncOptionsText] = useState("");

  const create = useMutation({
    mutationFn: () => {
      const lines = syncOptionsText
        .split("\n")
        .map((l) => l.trim())
        .filter(Boolean);
      return api.createApp({
        name,
        source: { repoUrl, path, targetRevision },
        destination: { cluster, namespace },
        syncPolicy: {
          ...(autoSync ? { automated: { prune: false, selfHeal: false } } : {}),
          ...(createNamespace ? { createNamespace: true } : {}),
          ...(lines.length > 0 ? { syncOptions: lines } : {}),
        },
      });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["apps"] });
      onClose();
    },
  });

  return (
    <div className="fixed inset-0 bg-black/55 flex items-center justify-center z-50 p-4">
      <div className="w-[480px] max-h-[90vh] overflow-y-auto rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] p-5 shadow-2xl space-y-3">
        <h2 className="text-lg font-semibold text-[var(--color-text)]">New application</h2>
        <Field label="Name">
          <input className="dialog-input" value={name} onChange={(e) => setName(e.target.value)} />
        </Field>
        <Field label="Repository">
          <select className="dialog-input" value={repoUrl} onChange={(e) => setRepoUrl(e.target.value)}>
            <option value="">— select repository —</option>
            {repos?.map((r) => (
              <option key={r.id} value={r.url}>
                {r.url}
              </option>
            ))}
          </select>
        </Field>
        <Field label="Path">
          <input className="dialog-input" value={path} onChange={(e) => setPath(e.target.value)} />
          <p className="mt-1.5 text-xs text-[var(--color-text-muted)] leading-relaxed">
            One application syncs exactly this folder. A Helm umbrella chart includes all enabled
            subcharts—use a leaf chart path (e.g. <code className="text-[var(--color-text)]">samples/hello-world</code>) and
            create another application if you want a separate deployable unit.
          </p>
        </Field>
        <Field label="Revision">
          <input className="dialog-input" value={targetRevision} onChange={(e) => setTargetRevision(e.target.value)} />
        </Field>
        <Field label="Cluster">
          <select className="dialog-input" value={cluster} onChange={(e) => setCluster(e.target.value)}>
            {clusters?.map((c) => (
              <option key={c.id} value={c.name}>
                {c.name}
              </option>
            ))}
          </select>
        </Field>
        <Field label="Namespace">
          <input className="dialog-input" value={namespace} onChange={(e) => setNamespace(e.target.value)} />
        </Field>
        <label className="flex items-center gap-2 text-sm text-[var(--color-text)]">
          <input type="checkbox" checked={autoSync} onChange={(e) => setAutoSync(e.target.checked)} />
          Enable automated sync
        </label>
        <label className="flex items-center gap-2 text-sm text-[var(--color-text)]">
          <input type="checkbox" checked={createNamespace} onChange={(e) => setCreateNamespace(e.target.checked)} />
          Create destination namespace on sync
        </label>
        <Field label="Sync options (Argo-style, optional)">
          <textarea
            className="dialog-input font-mono text-xs min-h-[4.5rem]"
            placeholder={"CreateNamespace=true\nValidate=false"}
            value={syncOptionsText}
            onChange={(e) => setSyncOptionsText(e.target.value)}
          />
          <p className="mt-1.5 text-xs text-[var(--color-text-muted)]">One option per line. Create namespace on sync still works via the checkbox above.</p>
        </Field>
        {create.error && <div className="text-sm text-red-400">{(create.error as Error).message}</div>}
        <div className="flex justify-end gap-2 pt-2">
          <button
            className="rounded-md border border-[var(--color-border)] bg-[var(--color-input-bg)] px-3 py-1.5 text-sm text-[var(--color-text)] hover:border-[var(--color-border-strong)]"
            onClick={onClose}
          >
            Cancel
          </button>
          <button
            className="rounded-md bg-[var(--color-accent)] px-3 py-1.5 text-sm font-medium text-[#0a0e14] hover:brightness-110 disabled:opacity-50"
            disabled={!name || !repoUrl || !namespace || create.isPending}
            onClick={() => create.mutate()}
          >
            Create
          </button>
        </div>
        <style>{`
          .dialog-input {
            width: 100%;
            border: 1px solid var(--color-border);
            border-radius: 0.375rem;
            padding: 0.375rem 0.5rem;
            font-size: 0.875rem;
            background: var(--color-input-bg);
            color: var(--color-text);
          }
        `}</style>
      </div>
    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block">
      <div className="text-xs text-[var(--color-text-muted)] mb-1">{label}</div>
      {children}
    </label>
  );
}
