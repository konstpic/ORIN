import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "../api/client";

export function RepositoriesPage() {
  const qc = useQueryClient();
  const { data: repos, isLoading } = useQuery({ queryKey: ["repos"], queryFn: api.listRepos });
  const [url, setUrl] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const create = useMutation({
    mutationFn: () => api.createRepo(url, username || undefined, password || undefined),
    onSuccess: () => {
      setUrl("");
      setUsername("");
      setPassword("");
      qc.invalidateQueries({ queryKey: ["repos"] });
    },
  });
  const del = useMutation({
    mutationFn: (id: string) => api.deleteRepo(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["repos"] }),
  });

  return (
    <div className="p-6 max-w-3xl flex-1 min-h-0 overflow-y-auto w-full">
      <h1 className="text-xl font-semibold mb-4 text-[var(--color-text)]">Repositories</h1>
      <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] p-4 mb-4 space-y-2">
        <h2 className="text-sm font-medium text-[var(--color-text)]">Register a Git repository</h2>
        <input
          className="repo-input"
          placeholder="https://github.com/org/repo.git"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
        />
        <div className="flex gap-2">
          <input
            className="repo-input flex-1"
            placeholder="Username (optional)"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
          />
          <input
            className="repo-input flex-1"
            placeholder="Password / PAT"
            value={password}
            type="password"
            onChange={(e) => setPassword(e.target.value)}
          />
        </div>
        {create.error && <div className="text-sm text-red-400">{(create.error as Error).message}</div>}
        <button
          className="rounded-md bg-[var(--color-accent)] px-3 py-1.5 text-sm font-medium text-[#0a0e14] hover:brightness-110 disabled:opacity-50"
          disabled={!url || create.isPending}
          onClick={() => create.mutate()}
        >
          Add repository
        </button>
        <style>{`
          .repo-input {
            width: 100%;
            border: 1px solid var(--color-border);
            border-radius: 0.375rem;
            padding: 0.375rem 0.5rem;
            font-size: 0.875rem;
            background: var(--color-input-bg);
            color: var(--color-text);
          }
          .repo-input::placeholder { color: var(--color-text-muted); }
        `}</style>
      </div>
      {isLoading ? (
        <div className="text-sm text-[var(--color-text-muted)]">Loading…</div>
      ) : (
        <table className="w-full text-sm bg-[var(--color-surface)] border border-[var(--color-border)] rounded-lg overflow-hidden">
          <thead className="text-left text-xs text-[var(--color-text-muted)] bg-[var(--color-surface-muted)]">
            <tr>
              <th className="px-3 py-2">URL</th>
              <th className="px-3 py-2">Creds</th>
              <th className="px-3 py-2">Created</th>
              <th></th>
            </tr>
          </thead>
          <tbody className="text-[var(--color-text)]">
            {repos?.map((r) => (
              <tr key={r.id} className="border-t border-[var(--color-border)]">
                <td className="px-3 py-2 break-all">{r.url}</td>
                <td className="px-3 py-2">{r.hasCreds ? "yes" : "no"}</td>
                <td className="px-3 py-2 text-xs">{new Date(r.createdAt).toLocaleString()}</td>
                <td className="px-3 py-2 text-right">
                  <button
                    className="text-xs text-red-400 hover:underline"
                    onClick={() => del.mutate(r.id)}
                  >
                    Remove
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
