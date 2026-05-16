import { ReactNode } from "react";
import { Link, NavLink } from "react-router-dom";
import { useAuth } from "../state/auth";
import {
  Bell,
  Boxes,
  Database,
  FolderGit,
  Settings as SettingsIcon,
  Shield,
} from "lucide-react";

export function Shell({ children }: { children: ReactNode }) {
  const setToken = useAuth((s) => s.setToken);
  return (
    <div className="flex h-full bg-[var(--color-canvas)]">
      <aside
        className="w-96 shrink-0 flex flex-col text-[var(--color-text)] shadow-lg overflow-y-auto"
        style={{ background: "var(--color-sidebar)" }}
      >
        <div className="px-4 py-5 border-b border-[var(--color-border)]">
          <Link to="/applications" className="text-lg font-semibold tracking-tight text-[var(--color-text)]">
            k8s-ui
          </Link>
          <div className="text-[11px] uppercase tracking-wider text-[var(--color-text-muted)] mt-1">GitOps</div>
        </div>
        <nav className="flex-1 px-2 py-4 space-y-0.5 text-sm">
          <SideLink to="/applications" label="Applications" icon={Boxes} />
          <div className="pt-4 pb-1 px-3 text-[10px] uppercase tracking-wider text-[var(--color-text-muted)] font-semibold">
            Settings
          </div>
          <SideLink to="/settings/repositories" label="Repositories" icon={FolderGit} />
          <SideLink to="/settings/clusters" label="Clusters" icon={Database} />
          <SideLink to="/settings/notifications" label="Notifications" icon={Bell} />
          <SideLink to="/settings/rbac" label="Access Control" icon={Shield} />
          <SideLink to="/settings/system" label="System" icon={SettingsIcon} />
        </nav>
        <button
          className="m-3 text-xs text-[var(--color-text-muted)] hover:text-[var(--color-text)] underline text-left"
          onClick={() => setToken(null)}
        >
          Sign out
        </button>
      </aside>
      <main className="flex-1 min-w-0 min-h-0 flex flex-col overflow-hidden">{children}</main>
    </div>
  );
}

function SideLink({ to, label, icon: Icon }: { to: string; label: string; icon?: typeof SettingsIcon }) {
  return (
    <NavLink
      to={to}
      end
      className={({ isActive }) =>
        `flex items-center gap-2.5 rounded-md px-3 py-2.5 font-medium transition-all duration-150 ${
          isActive
            ? "bg-[var(--color-sidebar-active)] text-[#0a0e14] shadow-sm"
            : "text-[var(--color-text-muted)] hover:bg-[var(--color-sidebar-hover)] hover:text-[var(--color-text)]"
        }`
      }
    >
      {Icon && <Icon className="size-4 shrink-0" strokeWidth={2} />}
      {label}
    </NavLink>
  );
}
