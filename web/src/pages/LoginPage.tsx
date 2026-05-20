import { FormEvent, useState } from "react";
import { useNavigate } from "react-router-dom";
import { api } from "../api/client";
import { useAuth } from "../state/auth";

export function LoginPage() {
  const [token, setToken] = useState("");
  const [err, setErr] = useState<string | null>(null);
  const setAuth = useAuth((s) => s.setToken);
  const nav = useNavigate();

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setErr(null);
    try {
      await api.login(token);
      setAuth(token);
      nav("/applications", { replace: true });
    } catch (e) {
      setErr("Invalid token");
    }
  }

  return (
    <div className="h-full flex items-center justify-center bg-[var(--color-canvas)]">
      <form
        onSubmit={onSubmit}
        className="w-80 rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] p-6 shadow-xl space-y-4"
      >
        <h1 className="text-lg font-semibold text-[var(--color-text)]">ORIN</h1>
        <p className="text-sm text-[var(--color-text-muted)]">Sign in with the admin token</p>
        <input
          type="password"
          value={token}
          onChange={(e) => setToken(e.target.value)}
          className="w-full rounded-md border border-[var(--color-border)] bg-[var(--color-input-bg)] px-3 py-2 text-sm text-[var(--color-text)] placeholder:text-[var(--color-text-muted)]"
          placeholder="Bearer token"
          autoFocus
        />
        {err && <div className="text-sm text-red-400">{err}</div>}
        <button
          type="submit"
          className="w-full rounded-md bg-[var(--color-accent)] px-3 py-2 text-sm font-medium text-[#0a0e14] hover:brightness-110"
        >
          Sign in
        </button>
      </form>
    </div>
  );
}
