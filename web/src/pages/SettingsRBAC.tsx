import { useState, useEffect, useCallback } from "react";
import { api } from "../api/client";
import { SettingsLayout } from "../components/SettingsLayout";
import type { Role, RoleBinding, UserInfo, PermissionInfo } from "../api/types";

export function SettingsRBAC() {
  return (
    <SettingsLayout title="Access Control" subtitle="Manage roles, users, and permissions">
      <RBACContent />
    </SettingsLayout>
  );
}

function RBACContent() {
  const [tab, setTab] = useState<"roles" | "users" | "bindings">("roles");
  const [roles, setRoles] = useState<Role[]>([]);
  const [users, setUsers] = useState<UserInfo[]>([]);
  const [bindings, setBindings] = useState<RoleBinding[]>([]);
  const [permissions, setPermissions] = useState<PermissionInfo[]>([]);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState<Record<string, boolean>>({ roles: true, users: true, bindings: true, perms: true });

  const loadRoles = useCallback(async () => {
    setLoading((prev) => ({ ...prev, roles: true }));
    try {
      const r = await api.listRoles();
      setRoles(r ?? []);
      setErrors((prev) => ({ ...prev, roles: "" }));
    } catch (e: unknown) {
      setErrors((prev) => ({ ...prev, roles: e instanceof Error ? e.message : "Failed to load" }));
    } finally {
      setLoading((prev) => ({ ...prev, roles: false }));
    }
  }, []);

  const loadUsers = useCallback(async () => {
    setLoading((prev) => ({ ...prev, users: true }));
    try {
      const u = await api.listUsers();
      setUsers(u ?? []);
      setErrors((prev) => ({ ...prev, users: "" }));
    } catch (e: unknown) {
      setErrors((prev) => ({ ...prev, users: e instanceof Error ? e.message : "Failed to load" }));
    } finally {
      setLoading((prev) => ({ ...prev, users: false }));
    }
  }, []);

  const loadBindings = useCallback(async () => {
    setLoading((prev) => ({ ...prev, bindings: true }));
    try {
      const b = await api.listRoleBindings();
      setBindings(b ?? []);
      setErrors((prev) => ({ ...prev, bindings: "" }));
    } catch (e: unknown) {
      setErrors((prev) => ({ ...prev, bindings: e instanceof Error ? e.message : "Failed to load" }));
    } finally {
      setLoading((prev) => ({ ...prev, bindings: false }));
    }
  }, []);

  const loadPermissions = useCallback(async () => {
    setLoading((prev) => ({ ...prev, perms: true }));
    try {
      setPermissions(await api.listPermissions());
    } catch {
      // non-critical
    } finally {
      setLoading((prev) => ({ ...prev, perms: false }));
    }
  }, []);

  useEffect(() => {
    loadRoles();
    loadUsers();
    loadBindings();
    loadPermissions();
  }, [loadRoles, loadUsers, loadBindings, loadPermissions]);

  const refreshAll = useCallback(() => {
    loadRoles();
    loadUsers();
    loadBindings();
  }, [loadRoles, loadUsers, loadBindings]);

  return (
    <div className="p-6 max-w-5xl flex-1 min-h-0 overflow-y-auto w-full">
      <div className="flex gap-1 mb-6 border-b border-[var(--color-border)]">
        {(["roles", "users", "bindings"] as const).map((t) => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className={`px-3 py-2 text-sm capitalize font-medium transition-colors ${
              tab === t
                ? "text-[var(--color-accent)] border-b-2 border-[var(--color-accent)]"
                : "text-[var(--color-text-muted)] hover:text-[var(--color-text)]"
            }`}
          >
            {t}
          </button>
        ))}
      </div>

      {tab === "roles" && <RolesTab roles={roles} permissions={permissions} onRefresh={refreshAll} loading={loading.roles} error={errors.roles} />}
      {tab === "users" && <UsersTab users={users} onRefresh={refreshAll} loading={loading.users} error={errors.users} />}
      {tab === "bindings" && (
        <BindingsTab bindings={bindings} roles={roles} users={users} onRefresh={refreshAll} loading={loading.bindings} error={errors.bindings} />
      )}

      <style>{rbacStyles}</style>
    </div>
  );
}

function RolesTab({ roles, permissions, onRefresh, loading, error }: { roles: Role[]; permissions: PermissionInfo[]; onRefresh: () => void; loading: boolean; error: string }) {
  const [showCreate, setShowCreate] = useState(false);

  if (loading) return <div className="text-sm text-[var(--color-text-muted)]">Loading roles…</div>;
  if (error) return <div className="text-sm text-red-400">{error}</div>;

  return (
    <div>
      <div className="flex justify-between items-center mb-4">
        <p className="text-sm text-[var(--color-text-muted)]">Roles define what actions users can perform. Built-in roles cannot be modified.</p>
        <button onClick={() => setShowCreate(true)} className="btn-accent">
          Create Role
        </button>
      </div>

      <div className="space-y-3">
        {roles.map((role) => (
          <div key={role.id} className="rounded-lg border border-[var(--color-border)] bg-[var(--color-surface)] p-4">
            <div className="flex items-center gap-3 mb-2">
              <span className="font-semibold text-[var(--color-text)]">{role.displayName}</span>
              {role.builtIn && (
                <span className="text-xs bg-[var(--color-surface-muted)] text-[var(--color-text-muted)] px-2 py-0.5 rounded">built-in</span>
              )}
              <span className="text-[var(--color-text-muted)] text-sm ml-auto font-mono">{role.name}</span>
            </div>
            {role.description && (
              <p className="text-sm text-[var(--color-text-muted)] mb-2">{role.description}</p>
            )}
            <div className="flex flex-wrap gap-1">
              {role.permissions.map((p: string) => (
                <span key={p} className="text-xs bg-[var(--color-surface-muted)] text-[var(--color-text-muted)] px-2 py-0.5 rounded">
                  {p}
                </span>
              ))}
            </div>
          </div>
        ))}
      </div>

      {showCreate && (
        <RoleForm permissions={permissions} onClose={() => setShowCreate(false)} onSuccess={() => { setShowCreate(false); onRefresh(); }} />
      )}
    </div>
  );
}

function RoleForm({ permissions, onClose, onSuccess, initial }: {
  permissions: PermissionInfo[]; onClose: () => void; onSuccess: () => void; initial?: Role;
}) {
  const [name, setName] = useState(initial?.name ?? "");
  const [displayName, setDisplayName] = useState(initial?.displayName ?? "");
  const [description, setDescription] = useState(initial?.description ?? "");
  const [selectedPerms, setSelectedPerms] = useState<Set<string>>(new Set(initial?.permissions ?? []));
  const [saving, setSaving] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const grouped = permissions.reduce<Record<string, PermissionInfo[]>>((acc, p) => {
    (acc[p.category] ??= []).push(p);
    return acc;
  }, {});

  const togglePerm = (id: string) => {
    setSelectedPerms((prev) => { const n = new Set(prev); n.has(id) ? n.delete(id) : n.add(id); return n; });
  };

  const toggleCategory = (category: string) => {
    const catPerms = grouped[category]?.map((p) => p.id) ?? [];
    const all = catPerms.every((p) => selectedPerms.has(p));
    setSelectedPerms((prev) => {
      const n = new Set(prev);
      catPerms.forEach((p) => { all ? n.delete(p) : n.add(p); });
      return n;
    });
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setErr(null);
    try {
      if (initial) {
        await api.updateRole(initial.id, { displayName, description, permissions: Array.from(selectedPerms) });
      } else {
        await api.createRole({ name, displayName, description, permissions: Array.from(selectedPerms) });
      }
      onSuccess();
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : "Failed to save role");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
      <div className="bg-[var(--color-surface)] border border-[var(--color-border)] rounded-lg p-6 w-full max-w-2xl max-h-[80vh] overflow-y-auto">
        <h2 className="text-lg font-semibold mb-4 text-[var(--color-text)]">{initial ? "Edit Role" : "Create Role"}</h2>
        {err && <div className="mb-4 text-sm text-red-400">{err}</div>}
        <form onSubmit={handleSubmit} className="space-y-4">
          {!initial && (
            <div>
              <label className="block text-sm text-[var(--color-text-muted)] mb-1">Role Name</label>
              <input type="text" value={name} onChange={(e) => setName(e.target.value)} className="rbac-input" placeholder="e.g., developer" required />
            </div>
          )}
          <div>
            <label className="block text-sm text-[var(--color-text-muted)] mb-1">Display Name</label>
            <input type="text" value={displayName} onChange={(e) => setDisplayName(e.target.value)} className="rbac-input" placeholder="e.g., Developer" required />
          </div>
          <div>
            <label className="block text-sm text-[var(--color-text-muted)] mb-1">Description</label>
            <input type="text" value={description} onChange={(e) => setDescription(e.target.value)} className="rbac-input" placeholder="What this role can do" />
          </div>
          <div>
            <label className="block text-sm text-[var(--color-text-muted)] mb-2">Permissions</label>
            <div className="space-y-3 max-h-60 overflow-y-auto">
              {Object.entries(grouped).map(([category, perms]) => {
                const allSelected = perms.every((p) => selectedPerms.has(p.id));
                return (
                  <div key={category}>
                    <button type="button" onClick={() => toggleCategory(category)} className="text-sm font-medium text-[var(--color-text)] mb-1 flex items-center gap-2">
                      <span className={`w-4 h-4 border rounded flex items-center justify-center ${allSelected ? "bg-[var(--color-accent)] border-[var(--color-accent)]" : "border-[var(--color-border)]"}`}>
                        {allSelected && <span className="text-[#0a0e14] text-xs">✓</span>}
                      </span>
                      {category}
                    </button>
                    <div className="flex flex-wrap gap-1 ml-6">
                      {perms.map((p) => (
                        <button key={p.id} type="button" onClick={() => togglePerm(p.id)} title={p.description}
                          className={`text-xs px-2 py-0.5 rounded border transition-colors ${
                            selectedPerms.has(p.id) ? "bg-[var(--color-accent)]/20 border-[var(--color-accent)] text-[var(--color-accent)]" : "bg-[var(--color-surface-muted)] border-[var(--color-border)] text-[var(--color-text-muted)]"
                          }`}>
                          {p.id.split(":")[1]}
                        </button>
                      ))}
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
          <div className="flex gap-2 justify-end pt-2">
            <button type="button" onClick={onClose} className="px-3 py-1.5 text-sm text-[var(--color-text-muted)] hover:text-[var(--color-text)]">Cancel</button>
            <button type="submit" disabled={saving} className="btn-accent disabled:opacity-50">{saving ? "Saving…" : "Save"}</button>
          </div>
        </form>
      </div>
    </div>
  );
}

function UsersTab({ users, onRefresh, loading, error }: { users: UserInfo[]; onRefresh: () => void; loading: boolean; error: string }) {
  const [showCreate, setShowCreate] = useState(false);

  if (loading) return <div className="text-sm text-[var(--color-text-muted)]">Loading users…</div>;
  if (error) return <div className="text-sm text-red-400">{error}</div>;

  const handleDelete = async (u: UserInfo) => {
    if (!confirm(`Delete user ${u.email}?`)) return;
    try { await api.deleteUser(u.id); onRefresh(); } catch (e: unknown) { alert(e instanceof Error ? e.message : "Failed to delete"); }
  };

  return (
    <div>
      <div className="flex justify-between items-center mb-4">
        <p className="text-sm text-[var(--color-text-muted)]">Manage users and their access tokens.</p>
        <button onClick={() => setShowCreate(true)} className="btn-accent">Create User</button>
      </div>

      <table className="w-full text-sm bg-[var(--color-surface)] border border-[var(--color-border)] rounded-lg overflow-hidden">
        <thead className="text-left text-xs text-[var(--color-text-muted)] bg-[var(--color-surface-muted)]">
          <tr>
            <th className="px-3 py-2">Email</th>
            <th className="px-3 py-2">Display Name</th>
            <th className="px-3 py-2">Role</th>
            <th className="px-3 py-2">Status</th>
            <th className="px-3 py-2"></th>
          </tr>
        </thead>
        <tbody className="text-[var(--color-text)]">
          {users.map((u) => (
            <tr key={u.id} className="border-t border-[var(--color-border)]">
              <td className="px-3 py-2">{u.email}</td>
              <td className="px-3 py-2 text-[var(--color-text-muted)]">{u.displayName || "-"}</td>
              <td className="px-3 py-2"><span className="bg-[var(--color-surface-muted)] px-2 py-0.5 rounded text-xs">{u.role}</span></td>
              <td className="px-3 py-2">
                <span className={`px-2 py-0.5 rounded text-xs ${u.active ? "bg-green-900/50 text-green-400" : "bg-red-900/50 text-red-400"}`}>{u.active ? "active" : "inactive"}</span>
              </td>
              <td className="px-3 py-2 text-right">
                <button className="text-xs text-red-400 hover:underline" onClick={() => handleDelete(u)}>Remove</button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      {showCreate && <UserForm onClose={() => setShowCreate(false)} onSuccess={() => { setShowCreate(false); onRefresh(); }} />}
    </div>
  );
}

function UserForm({ onClose, onSuccess }: { onClose: () => void; onSuccess: () => void }) {
  const [email, setEmail] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [role, setRole] = useState("viewer");
  const [token, setToken] = useState("");
  const [saving, setSaving] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true); setErr(null);
    try { await api.createUser({ email, displayName: displayName || email, role, token }); onSuccess(); }
    catch (e: unknown) { setErr(e instanceof Error ? e.message : "Failed to create user"); }
    finally { setSaving(false); }
  };

  return (
    <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
      <div className="bg-[var(--color-surface)] border border-[var(--color-border)] rounded-lg p-6 w-full max-w-md">
        <h2 className="text-lg font-semibold mb-4 text-[var(--color-text)]">Create User</h2>
        {err && <div className="mb-4 text-sm text-red-400">{err}</div>}
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm text-[var(--color-text-muted)] mb-1">Email</label>
            <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} className="rbac-input" required />
          </div>
          <div>
            <label className="block text-sm text-[var(--color-text-muted)] mb-1">Display Name</label>
            <input type="text" value={displayName} onChange={(e) => setDisplayName(e.target.value)} className="rbac-input" placeholder={email} />
          </div>
          <div>
            <label className="block text-sm text-[var(--color-text-muted)] mb-1">Role</label>
            <select value={role} onChange={(e) => setRole(e.target.value)} className="rbac-input">
              <option value="admin">Admin</option><option value="editor">Editor</option><option value="viewer">Viewer</option>
            </select>
          </div>
          <div>
            <label className="block text-sm text-[var(--color-text-muted)] mb-1">Access Token</label>
            <input type="text" value={token} onChange={(e) => setToken(e.target.value)} className="rbac-input font-mono text-sm" placeholder="User's bearer token" required />
          </div>
          <div className="flex gap-2 justify-end pt-2">
            <button type="button" onClick={onClose} className="px-3 py-1.5 text-sm text-[var(--color-text-muted)] hover:text-[var(--color-text)]">Cancel</button>
            <button type="submit" disabled={saving} className="btn-accent disabled:opacity-50">{saving ? "Creating…" : "Create"}</button>
          </div>
        </form>
      </div>
    </div>
  );
}

function BindingsTab({ bindings, roles, users, onRefresh, loading, error }: {
  bindings: RoleBinding[]; roles: Role[]; users: UserInfo[]; onRefresh: () => void; loading: boolean; error: string;
}) {
  const [showCreate, setShowCreate] = useState(false);

  const roleName = (roleId: string) => { const r = roles.find((x) => x.id === roleId); return r?.displayName ?? r?.name ?? roleId; };
  const userEmail = (userId: string) => users.find((u) => u.id === userId)?.email ?? userId;

  const handleDelete = async (id: string) => {
    if (!confirm("Delete this role binding?")) return;
    try { await api.deleteRoleBinding(id); onRefresh(); } catch (e: unknown) { alert(e instanceof Error ? e.message : "Failed to delete"); }
  };

  if (loading) return <div className="text-sm text-[var(--color-text-muted)]">Loading bindings…</div>;
  if (error) return <div className="text-sm text-red-400">{error}</div>;

  return (
    <div>
      <div className="flex justify-between items-center mb-4">
        <p className="text-sm text-[var(--color-text-muted)]">Assign roles to users, optionally scoped to projects.</p>
        <button onClick={() => setShowCreate(true)} className="btn-accent">Create Binding</button>
      </div>

      <table className="w-full text-sm bg-[var(--color-surface)] border border-[var(--color-border)] rounded-lg overflow-hidden">
        <thead className="text-left text-xs text-[var(--color-text-muted)] bg-[var(--color-surface-muted)]">
          <tr>
            <th className="px-3 py-2">User</th>
            <th className="px-3 py-2">Role</th>
            <th className="px-3 py-2">Projects</th>
            <th className="px-3 py-2">Created</th>
            <th className="px-3 py-2"></th>
          </tr>
        </thead>
        <tbody className="text-[var(--color-text)]">
          {bindings.map((b) => (
            <tr key={b.id} className="border-t border-[var(--color-border)]">
              <td className="px-3 py-2">{userEmail(b.userId)}</td>
              <td className="px-3 py-2"><span className="bg-[var(--color-surface-muted)] px-2 py-0.5 rounded text-xs">{roleName(b.roleId)}</span></td>
              <td className="px-3 py-2 text-[var(--color-text-muted)]">{b.projects.length === 0 || b.projects.includes("*") ? "All" : b.projects.join(", ")}</td>
              <td className="px-3 py-2 text-xs text-[var(--color-text-muted)]">{new Date(b.createdAt).toLocaleString()}</td>
              <td className="px-3 py-2 text-right">
                <button className="text-xs text-red-400 hover:underline" onClick={() => handleDelete(b.id)}>Remove</button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      {showCreate && <BindingForm roles={roles} users={users} onClose={() => setShowCreate(false)} onSuccess={() => { setShowCreate(false); onRefresh(); }} />}
    </div>
  );
}

function BindingForm({ roles, users, onClose, onSuccess }: {
  roles: Role[]; users: UserInfo[]; onClose: () => void; onSuccess: () => void;
}) {
  const [userId, setUserId] = useState("");
  const [roleId, setRoleId] = useState("");
  const [projects, setProjects] = useState("");
  const [saving, setSaving] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true); setErr(null);
    try {
      const projs = projects ? projects.split(",").map((p) => p.trim()).filter(Boolean) : [];
      await api.createRoleBinding({ userId, roleId, projects: projs });
      onSuccess();
    } catch (e: unknown) { setErr(e instanceof Error ? e.message : "Failed to create binding"); }
    finally { setSaving(false); }
  };

  return (
    <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
      <div className="bg-[var(--color-surface)] border border-[var(--color-border)] rounded-lg p-6 w-full max-w-md">
        <h2 className="text-lg font-semibold mb-4 text-[var(--color-text)]">Create Role Binding</h2>
        {err && <div className="mb-4 text-sm text-red-400">{err}</div>}
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm text-[var(--color-text-muted)] mb-1">User</label>
            <select value={userId} onChange={(e) => setUserId(e.target.value)} className="rbac-input" required>
              <option value="">Select user</option>
              {users.map((u) => <option key={u.id} value={u.id}>{u.email}</option>)}
            </select>
          </div>
          <div>
            <label className="block text-sm text-[var(--color-text-muted)] mb-1">Role</label>
            <select value={roleId} onChange={(e) => setRoleId(e.target.value)} className="rbac-input" required>
              <option value="">Select role</option>
              {roles.map((r) => <option key={r.id} value={r.id}>{r.displayName}</option>)}
            </select>
          </div>
          <div>
            <label className="block text-sm text-[var(--color-text-muted)] mb-1">Projects (comma-separated, empty = all)</label>
            <input type="text" value={projects} onChange={(e) => setProjects(e.target.value)} className="rbac-input" placeholder="e.g., frontend, backend" />
          </div>
          <div className="flex gap-2 justify-end pt-2">
            <button type="button" onClick={onClose} className="px-3 py-1.5 text-sm text-[var(--color-text-muted)] hover:text-[var(--color-text)]">Cancel</button>
            <button type="submit" disabled={saving} className="btn-accent disabled:opacity-50">{saving ? "Creating…" : "Create"}</button>
          </div>
        </form>
      </div>
    </div>
  );
}

const rbacStyles = `
  .rbac-input {
    width: 100%;
    border: 1px solid var(--color-border);
    border-radius: 0.375rem;
    padding: 0.375rem 0.5rem;
    font-size: 0.875rem;
    background: var(--color-input-bg);
    color: var(--color-text);
  }
  .rbac-input::placeholder { color: var(--color-text-muted); }
  .btn-accent {
    border-radius: 0.375rem;
    background: var(--color-accent);
    padding: 0.375rem 0.75rem;
    font-size: 0.875rem;
    font-weight: 500;
    color: #0a0e14;
    transition: filter 150ms;
  }
  .btn-accent:hover { filter: brightness(110%); }
`;
