import { useQuery } from "@tanstack/react-query";
import { Editor } from "@monaco-editor/react";
import { api } from "../api/client";

export function ManifestsView({ name }: { name: string }) {
  const { data, isLoading, error } = useQuery({
    queryKey: ["app-manifests", name],
    queryFn: () => api.appManifests(name),
  });
  if (isLoading) return <div className="text-sm text-[var(--color-text-muted)]">Rendering…</div>;
  if (error) return <div className="text-sm text-red-400">{(error as Error).message}</div>;
  if (!data) return null;
  const yaml = data.manifests.map((m) => JSON.stringify(m, null, 2)).join("\n---\n");
  return (
    <div className="space-y-2">
      <div className="text-xs text-[var(--color-text-muted)]">
        Revision: <code className="text-[var(--color-accent)]">{data.revision}</code>
      </div>
      <div className="border border-[var(--color-border)] rounded-lg bg-[var(--color-surface-muted)] h-[70vh] overflow-hidden">
        <Editor
          height="100%"
          theme="vs-dark"
          language="yaml"
          value={yaml}
          options={{ readOnly: true, minimap: { enabled: false } }}
        />
      </div>
    </div>
  );
}
