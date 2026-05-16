import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Editor } from "@monaco-editor/react";
import {
  Code,
  Loader2,
  Plus,
  Trash2,
  X,
} from "lucide-react";
import { api } from "../api/client";

interface SyncHook {
  id: string;
  appId: string;
  name: string;
  phase: string;
  yaml: string;
  enabled: boolean;
  createdAt: string;
  updatedAt: string;
}

const PHASES = [
  { value: "PreSync", label: "PreSync", color: "bg-blue-500/20 text-blue-300" },
  { value: "PostSync", label: "PostSync", color: "bg-green-500/20 text-green-300" },
  { value: "SyncFail", label: "SyncFail", color: "bg-red-500/20 text-red-300" },
];

export function SyncHooksPanel({ appName }: { appName: string }) {
  const qc = useQueryClient();
  const [creating, setCreating] = useState(false);
  const [editId, setEditId] = useState<string | null>(null);
  const [form, setForm] = useState({ name: "", phase: "PreSync", yaml: "", enabled: true });
  const [expandedYaml, setExpandedYaml] = useState<string | null>(null);

  const { data: hooks, isLoading, error } = useQuery({
    queryKey: ["sync-hooks", appName],
    queryFn: () => api.get<SyncHook[]>(`/api/v1/applications/${encodeURIComponent(appName)}/hooks`),
  });

  const createMut = useMutation({
    mutationFn: (body: typeof form) =>
      api.post(`/api/v1/applications/${encodeURIComponent(appName)}/hooks`, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["sync-hooks", appName] });
      setCreating(false);
      setForm({ name: "", phase: "PreSync", yaml: "", enabled: true });
    },
  });

  const updateMut = useMutation({
    mutationFn: ({ id, body }: { id: string; body: typeof form }) =>
      api.put(`/api/v1/applications/${encodeURIComponent(appName)}/hooks/${id}`, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["sync-hooks", appName] });
      setEditId(null);
      setExpandedYaml(null);
    },
  });

  const deleteMut = useMutation({
    mutationFn: (id: string) =>
      api.del(`/api/v1/applications/${encodeURIComponent(appName)}/hooks/${id}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["sync-hooks", appName] });
      if (editId) setEditId(null);
      setExpandedYaml(null);
    },
  });

  const phaseColor = (phase: string) => PHASES.find((p) => p.value === phase)?.color ?? "bg-gray-500/20 text-gray-300";

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-sm font-semibold text-[var(--color-text)] flex items-center gap-2">
            <Code className="size-4" strokeWidth={2} />
            Resource Hooks
          </h3>
          <p className="text-xs text-[var(--color-text-muted)] mt-0.5">
            PreSync, PostSync, and SyncFail Job manifests
          </p>
        </div>
        {!creating && (
          <button
            type="button"
            className="inline-flex items-center gap-1.5 rounded-md bg-[var(--color-accent)] px-2.5 py-1.5 text-xs font-semibold text-[#0a0e14] hover:brightness-110 transition-all duration-150 hover:scale-[1.02] active:scale-[0.98]"
            onClick={() => setCreating(true)}
          >
            <Plus className="size-3.5" />
            Add hook
          </button>
        )}
      </div>

      {isLoading && (
        <div className="flex items-center gap-2 text-xs text-[var(--color-text-muted)] animate-pulse">
          <Loader2 className="size-3.5 animate-spin" />
          Loading hooks…
        </div>
      )}
      {error && <div className="text-xs text-red-400">{(error as Error).message}</div>}

      {!isLoading && hooks && hooks.length === 0 && !creating && (
        <div className="text-xs text-[var(--color-text-muted)]">
          No hooks configured. Add a Job manifest to run before/after sync or on failure.
        </div>
      )}

      {/* Create form */}
      {creating && (
        <HookForm
          form={form}
          onChange={setForm}
          onCancel={() => setCreating(false)}
          onSave={() => createMut.mutate(form)}
          saving={createMut.isPending}
        />
      )}

      {/* Hook list */}
      {(hooks as SyncHook[] | undefined)?.map((hook) =>
        editId === hook.id ? (
          <HookForm
            key={hook.id}
            form={{ name: hook.name, phase: hook.phase, yaml: hook.yaml, enabled: hook.enabled }}
            onChange={(f) => setForm(f)}
            onCancel={() => { setEditId(null); setExpandedYaml(null); }}
            onSave={() => updateMut.mutate({ id: hook.id, body: form })}
            saving={updateMut.isPending}
          />
        ) : (
          <div
            key={hook.id}
            className={`rounded-lg border overflow-hidden transition-all duration-150 ${
              hook.enabled
                ? "border-[var(--color-border)] bg-[var(--color-surface)]"
                : "border-[var(--color-border)] bg-[var(--color-surface-muted)] opacity-60"
            }`}
          >
            <div className="px-3 py-2.5 flex items-center gap-3">
              <Code className="size-4 shrink-0 text-[var(--color-text-muted)]" strokeWidth={2} />
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <span className="text-xs font-semibold text-[var(--color-text)]">{hook.name}</span>
                  <span className={`text-[10px] px-1.5 py-0.5 rounded font-medium ${phaseColor(hook.phase)}`}>
                    {hook.phase}
                  </span>
                  {!hook.enabled && (
                    <span className="text-[10px] text-[var(--color-text-muted)]">disabled</span>
                  )}
                </div>
              </div>
              <div className="flex items-center gap-1 shrink-0">
                <button
                  type="button"
                  className="p-1.5 rounded-md border border-[var(--color-border)] bg-[var(--color-input-bg)] text-[var(--color-text-muted)] hover:text-[var(--color-accent)] hover:border-[var(--color-border-strong)] transition-all duration-150"
                  title="View/edit YAML"
                  onClick={() => {
                    if (expandedYaml === hook.id) setExpandedYaml(null);
                    else setExpandedYaml(hook.id);
                    setEditId(hook.id);
                    setForm({ name: hook.name, phase: hook.phase, yaml: hook.yaml, enabled: hook.enabled });
                  }}
                >
                  <Code className="size-3.5" />
                </button>
                <button
                  type="button"
                  className="p-1.5 rounded-md border border-[var(--color-border)] bg-[var(--color-input-bg)] text-red-400 hover:text-red-300 hover:border-red-500/40 transition-all duration-150"
                  title="Delete"
                  onClick={() => deleteMut.mutate(hook.id)}
                >
                  <Trash2 className="size-3.5" />
                </button>
              </div>
            </div>
            {expandedYaml === hook.id && (
              <div className="border-t border-[var(--color-border)]">
                <Editor
                  height="200px"
                  theme="vs-dark"
                  language="yaml"
                  value={hook.yaml}
                  options={{ readOnly: true, minimap: { enabled: false }, wordWrap: "on", scrollBeyondLastLine: false }}
                />
              </div>
            )}
          </div>
        )
      )}
    </div>
  );
}

function HookForm({
  form,
  onChange,
  onCancel,
  onSave,
  saving,
}: {
  form: { name: string; phase: string; yaml: string; enabled: boolean };
  onChange: (f: typeof form) => void;
  onCancel: () => void;
  onSave: () => void;
  saving: boolean;
}) {
  return (
    <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] p-3 space-y-3">
      <div className="grid grid-cols-2 gap-2">
        <div>
          <label className="block text-[10px] uppercase text-[var(--color-text-muted)] mb-1">Name</label>
          <input
            type="text"
            value={form.name}
            onChange={(e) => onChange({ ...form, name: e.target.value })}
            placeholder="e.g. db-migrate"
            className="w-full rounded-md border border-[var(--color-border)] bg-[var(--color-input-bg)] px-2 py-1.5 text-xs text-[var(--color-text)]"
          />
        </div>
        <div>
          <label className="block text-[10px] uppercase text-[var(--color-text-muted)] mb-1">Phase</label>
          <select
            value={form.phase}
            onChange={(e) => onChange({ ...form, phase: e.target.value })}
            className="w-full rounded-md border border-[var(--color-border)] bg-[var(--color-input-bg)] px-2 py-1.5 text-xs text-[var(--color-text)]"
          >
            {PHASES.map((p) => (
              <option key={p.value} value={p.value}>{p.label}</option>
            ))}
          </select>
        </div>
      </div>
      <div>
        <label className="block text-[10px] uppercase text-[var(--color-text-muted)] mb-1">Manifest (YAML)</label>
        <Editor
          height="160px"
          theme="vs-dark"
          language="yaml"
          value={form.yaml}
          onChange={(v) => onChange({ ...form, yaml: v ?? "" })}
          options={{ minimap: { enabled: false }, wordWrap: "on", scrollBeyondLastLine: false }}
        />
      </div>
      <div className="flex items-center gap-2">
        <label className="inline-flex items-center gap-1.5 text-xs text-[var(--color-text)] cursor-pointer">
          <input
            type="checkbox"
            checked={form.enabled}
            onChange={(e) => onChange({ ...form, enabled: e.target.checked })}
          />
          Enabled
        </label>
      </div>
      <div className="flex justify-end gap-2 pt-1">
        <button
          type="button"
          className="rounded-md px-3 py-1.5 text-xs font-medium border border-[var(--color-border)] text-[var(--color-text-muted)] hover:text-[var(--color-text)] transition-all duration-150"
          onClick={onCancel}
        >
          Cancel
        </button>
        <button
          type="button"
          className="rounded-md px-3 py-1.5 text-xs font-semibold bg-[var(--color-accent)] text-[#0a0e14] hover:brightness-110 disabled:opacity-50 transition-all duration-150"
          onClick={onSave}
          disabled={saving || !form.name || !form.yaml}
        >
          {saving ? "Saving…" : "Save"}
        </button>
      </div>
    </div>
  );
}
