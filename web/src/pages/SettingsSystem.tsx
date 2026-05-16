import { useCallback, useEffect, useState } from "react";
import { api, ApiError } from "../api/client";
import type { SystemConfig } from "../api/types";
import { SettingsLayout } from "../components/SettingsLayout";

const HELP_TEXT: Record<string, string> = {
  reconcileWorkers: "Number of parallel reconciliation workers (1–50).",
  reconcileResync: "Interval for full status re-sync (e.g. 3m, 30s, 1h).",
  repoPollInterval: "How often to poll Git repositories for new commits.",
  repoRenderTimeout: "Maximum time allowed for manifest rendering (Kustomize/Helm).",
  syncApplyRetries: "Per-resource apply retry count on transient errors.",
  autoSyncGracePeriod: "Suppress auto-sync after a manual edit so changes persist and show as OutOfSync.",
  syncDenyRangeUtc: "Daily UTC window when sync is blocked, e.g. 22:00-06:00. Empty to disable.",
  appsCatalogRepoUrl: "Git URL for the declarative Apps Catalog (app-of-apps). Empty to disable.",
  appsCatalogPath: "Path inside the catalog repo to the apps.yaml file.",
  appsCatalogInterval: "How often to poll the Apps Catalog repo.",
};

const DEFAULTS: Record<string, string> = {
  reconcileWorkers: "10",
  reconcileResync: "3m",
  repoPollInterval: "3m",
  repoRenderTimeout: "60s",
  syncApplyRetries: "1",
  autoSyncGracePeriod: "30m",
  syncDenyRangeUtc: "",
  appsCatalogRepoUrl: "",
  appsCatalogPath: "k8s-ui/apps.yaml",
  appsCatalogInterval: "5m",
};

const SECTIONS = [
  {
    title: "Controller",
    description: "Reconciliation loop tuning",
    fields: ["reconcileWorkers", "reconcileResync"] as const,
  },
  {
    title: "Sync",
    description: "Sync execution behavior",
    fields: ["syncApplyRetries", "autoSyncGracePeriod", "syncDenyRangeUtc"] as const,
  },
  {
    title: "Repository Server",
    description: "Git manifest rendering settings",
    fields: ["repoPollInterval", "repoRenderTimeout"] as const,
  },
  {
    title: "Apps Catalog",
    description: "Declarative app-of-apps registry",
    fields: ["appsCatalogRepoUrl", "appsCatalogPath", "appsCatalogInterval"] as const,
  },
] as const;

export function SettingsSystem() {
  const [config, setConfig] = useState<SystemConfig | null>(null);
  const [draft, setDraft] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const c = await api.getSystemConfig();
      setConfig(c);
      const d: Record<string, string> = {};
      for (const key of Object.keys(DEFAULTS)) {
        const v = (c as unknown as Record<string, unknown>)[key];
        d[key] = v !== undefined && v !== "" ? String(v) : DEFAULTS[key];
      }
      setDraft(d);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Failed to load system config");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const handleChange = (key: string, value: string) => {
    setDraft((prev) => ({ ...prev, [key]: value }));
    setSaved(false);
  };

  const handleSave = async () => {
    setSaving(true);
    setError("");
    try {
      await api.updateSystemConfig(draft);
      setSaved(true);
      load();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Failed to save");
    } finally {
      setSaving(false);
    }
  };

  const handleReset = (key: string) => {
    handleChange(key, DEFAULTS[key] ?? "");
  };

  if (loading) return <SettingsLayout title="System" subtitle="Loading…"><div className="p-8 text-sm text-[var(--color-text-muted)]">Loading…</div></SettingsLayout>;

  return (
    <SettingsLayout title="System" subtitle="System configuration and runtime settings">
      <div className="flex flex-col h-full overflow-y-auto p-8">
        <div className="max-w-3xl space-y-8">
          {SECTIONS.map((sec) => (
            <div key={sec.title}>
              <h3 className="text-base font-semibold text-[var(--color-text)] mb-1">{sec.title}</h3>
              <p className="text-sm text-[var(--color-text-muted)] mb-4">{sec.description}</p>
              <div className="space-y-4 bg-[var(--color-surface)] border border-[var(--color-border)] rounded-lg p-5">
                {sec.fields.map((key) => (
                  <ConfigField
                    key={key}
                    name={key as keyof SystemConfig}
                    value={draft[key] ?? ""}
                    onChange={(v) => handleChange(key, v)}
                    onReset={() => handleReset(key)}
                  />
                ))}
              </div>
            </div>
          ))}

          {error && (
            <div className="text-sm text-red-400 bg-red-400/10 border border-red-400/20 rounded-lg px-4 py-3">
              {error}
            </div>
          )}
          {saved && (
            <div className="text-sm text-green-400 bg-green-400/10 border border-green-400/20 rounded-lg px-4 py-3">
              Settings saved successfully.
            </div>
          )}

          <div className="flex gap-3 pt-2">
            <button
              onClick={handleSave}
              disabled={saving}
              className="px-4 py-2 text-sm font-medium rounded-md bg-[var(--color-accent)] text-white disabled:opacity-50 hover:bg-[var(--color-accent)]/90 transition"
            >
              {saving ? "Saving…" : "Save"}
            </button>
            <button
              onClick={() => load()}
              className="px-4 py-2 text-sm font-medium rounded-md border border-[var(--color-border)] text-[var(--color-text-muted)] hover:text-[var(--color-text)] hover:bg-[var(--color-sidebar-hover)] transition"
            >
              Discard changes
            </button>
          </div>
        </div>
      </div>
    </SettingsLayout>
  );
}

function ConfigField({
  name,
  value,
  onChange,
  onReset,
}: {
  name: string;
  value: string;
  onChange: (v: string) => void;
  onReset: () => void;
}) {
  const label = camelToTitle(name);
  const help = HELP_TEXT[name] ?? "";
  const isDefault = value === (DEFAULTS[name] ?? "");

  const isDuration = /Interval|Timeout|GracePeriod|Resync/.test(name);

  return (
    <div className="flex flex-col sm:flex-row sm:items-start gap-3 sm:gap-6">
      <label className="sm:w-48 shrink-0 text-sm font-medium text-[var(--color-text)] pt-0.5">{label}</label>
      <div className="flex-1 space-y-1.5">
        <input
          type={/Workers|Retries/.test(name) ? "number" : "text"}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={DEFAULTS[name] ?? ""}
          className="w-full max-w-xs bg-[var(--color-bg)] border border-[var(--color-border)] rounded-md px-3 py-2 text-sm text-[var(--color-text)] placeholder:text-[var(--color-text-muted)] focus:outline-none focus:ring-1 focus:ring-[var(--color-accent)]/40"
        />
        {help && <p className="text-xs text-[var(--color-text-muted)]">{help}</p>}
        {!isDefault && (
          <button onClick={onReset} className="text-xs text-[var(--color-accent)] hover:underline">
            Reset to default ({DEFAULTS[name]})
          </button>
        )}
      </div>
    </div>
  );
}

function camelToTitle(s: string): string {
  return s
    .replace(/([A-Z])/g, " $1")
    .replace(/^./, (c) => c.toUpperCase());
}
