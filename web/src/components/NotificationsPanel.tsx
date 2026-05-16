import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  Bell,
  BellOff,
  Copy,
  Loader2,
  Plus,
  Send,
  Trash2,
  X,
} from "lucide-react";
import { api } from "../api/client";

interface NotificationConfig {
  id: string;
  appId: string;
  name: string;
  type: string;
  url: string;
  events: string[];
  enabled: boolean;
  createdAt: string;
}

const ALL_EVENTS = [
  { value: "sync_succeeded", label: "Sync succeeded" },
  { value: "sync_failed", label: "Sync failed" },
  { value: "health_degraded", label: "Health degraded" },
  { value: "health_recovered", label: "Health recovered" },
  { value: "app_out_of_sync", label: "App out of sync" },
  { value: "app_synced", label: "App synced" },
];

export function NotificationsPanel({ appName }: { appName: string }) {
  const qc = useQueryClient();
  const isGlobal = appName === "__global__";
  const basePath = isGlobal ? "/api/v1/notifications" : `/api/v1/applications/${encodeURIComponent(appName)}/notifications`;
  const [creating, setCreating] = useState(false);
  const [testingUrl, setTestingUrl] = useState<string | null>(null);
  const [testResult, setTestResult] = useState<{ ok: boolean; msg: string } | null>(null);
  const [editId, setEditId] = useState<string | null>(null);
  const [form, setForm] = useState({ name: "", type: "webhook", url: "", events: ["sync_failed"], enabled: true });

  const { data: configs, isLoading, error } = useQuery<NotificationConfig[]>({
    queryKey: ["notifications", appName],
    queryFn: () => api.get(basePath),
  });

  const createMut = useMutation({
    mutationFn: (body: typeof form) => api.post(basePath, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["notifications", appName] });
      setCreating(false);
      setForm({ name: "", type: "webhook", url: "", events: ["sync_failed"], enabled: true });
    },
  });

  const updateMut = useMutation({
    mutationFn: ({ id, body }: { id: string; body: typeof form }) =>
      api.put(`${basePath}/${id}`, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["notifications", appName] });
      setEditId(null);
    },
  });

  const deleteMut = useMutation({
    mutationFn: (id: string) => api.del(`${basePath}/${id}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["notifications", appName] });
      if (editId) setEditId(null);
    },
  });

  const testMut = useMutation({
    mutationFn: (url: string) =>
      api.post("/api/v1/notifications/test", { url }),
    onSuccess: () => setTestResult({ ok: true, msg: "Test notification sent successfully" }),
    onError: (e: Error) => setTestResult({ ok: false, msg: e.message }),
  });

  const handleTest = (url: string) => {
    setTestingUrl(url);
    setTestResult(null);
    testMut.mutate(url, { onSettled: () => setTestingUrl(null) });
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-sm font-semibold text-[var(--color-text)] flex items-center gap-2">
            <Bell className="size-4" strokeWidth={2} />
            {isGlobal ? "Global Notifications" : "Notifications"}
          </h3>
          <p className="text-xs text-[var(--color-text-muted)] mt-0.5">
            {isGlobal
              ? "Webhook alerts for all applications across the platform"
              : "Webhook alerts on sync and health events"}
          </p>
        </div>
        {!creating && (
          <button
            type="button"
            className="inline-flex items-center gap-1.5 rounded-md bg-[var(--color-accent)] px-2.5 py-1.5 text-xs font-semibold text-[#0a0e14] hover:brightness-110 transition-all duration-150 hover:scale-[1.02] active:scale-[0.98]"
            onClick={() => setCreating(true)}
          >
            <Plus className="size-3.5" />
            Add webhook
          </button>
        )}
      </div>

      {testResult && (
        <div className={`rounded-lg border px-3 py-2 text-xs flex items-center gap-2 ${
          testResult.ok
            ? "border-emerald-500/40 bg-emerald-500/5 text-emerald-300"
            : "border-red-500/40 bg-red-500/5 text-red-300"
        }`}>
          {testResult.msg}
          <button className="ml-auto" onClick={() => setTestResult(null)}><X className="size-3.5" /></button>
        </div>
      )}

      {isLoading && (
        <div className="flex items-center gap-2 text-xs text-[var(--color-text-muted)] animate-pulse">
          <Loader2 className="size-3.5 animate-spin" />
          Loading notifications…
        </div>
      )}
      {error && <div className="text-xs text-red-400">{(error as Error)?.message}</div>}

      {!isLoading && configs && (configs as NotificationConfig[]).length === 0 && !creating && (
        <div className="text-xs text-[var(--color-text-muted)]">
          {isGlobal
            ? "No global notification webhooks configured."
            : "No notification webhooks configured. Add one to receive alerts on sync and health events."}
        </div>
      )}

      {creating && (
        <ConfigForm
          form={form}
          onChange={setForm}
          onCancel={() => setCreating(false)}
          onSave={() => createMut.mutate(form)}
          onTest={handleTest}
          testing={testingUrl === form.url}
          saving={createMut.isPending}
        />
      )}

      {(configs as NotificationConfig[] | undefined)?.map((cfg) =>
        editId === cfg.id ? (
          <ConfigForm
            key={cfg.id}
            form={{ name: cfg.name, type: cfg.type, url: cfg.url, events: cfg.events, enabled: cfg.enabled }}
            onChange={(f) => setForm(f)}
            onCancel={() => setEditId(null)}
            onSave={() => updateMut.mutate({ id: cfg.id, body: form })}
            onTest={handleTest}
            testing={testingUrl === cfg.url}
            saving={updateMut.isPending}
          />
        ) : (
          <div
            key={cfg.id}
            className={`rounded-lg border overflow-hidden transition-all duration-150 ${
              cfg.enabled
                ? "border-[var(--color-border)] bg-[var(--color-surface)]"
                : "border-[var(--color-border)] bg-[var(--color-surface-muted)] opacity-60"
            }`}
          >
            <div className="px-3 py-2.5 flex items-center gap-3">
              {cfg.enabled ? (
                <Bell className="size-4 shrink-0 text-[var(--color-accent)]" strokeWidth={2} />
              ) : (
                <BellOff className="size-4 shrink-0 text-[var(--color-text-muted)]" strokeWidth={2} />
              )}
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <span className="text-xs font-semibold text-[var(--color-text)]">{cfg.name}</span>
                  <span className="text-[10px] uppercase tracking-wide text-[var(--color-text-muted)] bg-[var(--color-surface-muted)] px-1.5 py-0.5 rounded">
                    {cfg.type}
                  </span>
                  {!cfg.enabled && (
                    <span className="text-[10px] text-[var(--color-text-muted)]">disabled</span>
                  )}
                </div>
                <div className="font-mono text-[10px] text-[var(--color-text-muted)] truncate mt-0.5">{cfg.url}</div>
                <div className="flex flex-wrap gap-1 mt-1">
                  {cfg.events.map((e) => (
                    <span key={e} className="text-[10px] bg-[var(--color-accent-muted)] text-[var(--color-accent)] px-1.5 py-0.5 rounded">
                      {e}
                    </span>
                  ))}
                </div>
              </div>
              <div className="flex items-center gap-1 shrink-0">
                <button
                  type="button"
                  className="p-1.5 rounded-md border border-[var(--color-border)] bg-[var(--color-input-bg)] text-[var(--color-text-muted)] hover:text-[var(--color-accent)] hover:border-[var(--color-border-strong)] transition-all duration-150"
                  title="Test webhook"
                  onClick={() => handleTest(cfg.url)}
                  disabled={testingUrl === cfg.url}
                >
                  {testingUrl === cfg.url ? (
                    <Loader2 className="size-3.5 animate-spin" />
                  ) : (
                    <Send className="size-3.5" />
                  )}
                </button>
                <button
                  type="button"
                  className="p-1.5 rounded-md border border-[var(--color-border)] bg-[var(--color-input-bg)] text-[var(--color-text-muted)] hover:text-[var(--color-accent)] hover:border-[var(--color-border-strong)] transition-all duration-150"
                  title="Edit"
                  onClick={() => { setEditId(cfg.id); setForm({ name: cfg.name, type: cfg.type, url: cfg.url, events: cfg.events, enabled: cfg.enabled }); }}
                >
                  <Copy className="size-3.5" />
                </button>
                <button
                  type="button"
                  className="p-1.5 rounded-md border border-[var(--color-border)] bg-[var(--color-input-bg)] text-red-400 hover:text-red-300 hover:border-red-500/40 transition-all duration-150"
                  title="Delete"
                  onClick={() => deleteMut.mutate(cfg.id)}
                >
                  <Trash2 className="size-3.5" />
                </button>
              </div>
            </div>
          </div>
        )
      )}
    </div>
  );
}

function ConfigForm({
  form,
  onChange,
  onCancel,
  onSave,
  onTest,
  testing,
  saving,
}: {
  form: { name: string; type: string; url: string; events: string[]; enabled: boolean };
  onChange: (f: typeof form) => void;
  onCancel: () => void;
  onSave: () => void;
  onTest: (url: string) => void;
  testing: boolean;
  saving: boolean;
}) {
  const toggleEvent = (e: string) => {
    const events = form.events.includes(e)
      ? form.events.filter((x) => x !== e)
      : [...form.events, e];
    onChange({ ...form, events });
  };

  return (
    <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] p-3 space-y-3">
      <div className="grid grid-cols-2 gap-2">
        <div>
          <label className="block text-[10px] uppercase text-[var(--color-text-muted)] mb-1">Name</label>
          <input
            type="text"
            value={form.name}
            onChange={(e) => onChange({ ...form, name: e.target.value })}
            placeholder="e.g. Slack alerts"
            className="w-full rounded-md border border-[var(--color-border)] bg-[var(--color-input-bg)] px-2 py-1.5 text-xs text-[var(--color-text)]"
          />
        </div>
        <div>
          <label className="block text-[10px] uppercase text-[var(--color-text-muted)] mb-1">Type</label>
          <select
            value={form.type}
            onChange={(e) => onChange({ ...form, type: e.target.value })}
            className="w-full rounded-md border border-[var(--color-border)] bg-[var(--color-input-bg)] px-2 py-1.5 text-xs text-[var(--color-text)]"
          >
            <option value="webhook">Generic webhook</option>
            <option value="slack">Slack</option>
          </select>
        </div>
      </div>
      <div>
        <label className="block text-[10px] uppercase text-[var(--color-text-muted)] mb-1">Webhook URL</label>
        <div className="flex gap-1.5">
          <input
            type="url"
            value={form.url}
            onChange={(e) => onChange({ ...form, url: e.target.value })}
            placeholder="https://hooks.slack.com/services/..."
            className="flex-1 rounded-md border border-[var(--color-border)] bg-[var(--color-input-bg)] px-2 py-1.5 text-xs text-[var(--color-text)] font-mono"
          />
          <button
            type="button"
            className="rounded-md border border-[var(--color-border)] bg-[var(--color-input-bg)] px-2 text-[var(--color-text-muted)] hover:text-[var(--color-accent)] transition-colors"
            onClick={() => onTest(form.url)}
            disabled={!form.url || testing}
            title="Test webhook"
          >
            {testing ? <Loader2 className="size-3.5 animate-spin" /> : <Send className="size-3.5" />}
          </button>
        </div>
      </div>
      <div>
        <label className="block text-[10px] uppercase text-[var(--color-text-muted)] mb-1">Events</label>
        <div className="flex flex-wrap gap-1.5">
          {ALL_EVENTS.map((e) => (
            <button
              key={e.value}
              type="button"
              className={`rounded-full px-2.5 py-0.5 text-xs font-medium transition-all duration-150 ${
                form.events.includes(e.value)
                  ? "bg-[var(--color-accent)] text-[#0a0e14]"
                  : "bg-[var(--color-surface-muted)] text-[var(--color-text-muted)] hover:text-[var(--color-text)] border border-[var(--color-border)]"
              }`}
              onClick={() => toggleEvent(e.value)}
            >
              {e.label}
            </button>
          ))}
        </div>
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
          disabled={saving || !form.name || !form.url || form.events.length === 0}
        >
          {saving ? "Saving…" : "Save"}
        </button>
      </div>
    </div>
  );
}
