import { ReactNode } from "react";

export function SettingsLayout({
  title,
  subtitle,
  children,
}: {
  title: string;
  subtitle: string;
  children: ReactNode;
}) {
  return (
    <div className="flex flex-col h-full overflow-hidden">
      <header className="shrink-0 px-8 py-6 border-b border-[var(--color-border)] bg-[var(--color-surface)]">
        <h1 className="text-xl font-semibold text-[var(--color-text)]">{title}</h1>
        <p className="text-sm text-[var(--color-text-muted)] mt-1">{subtitle}</p>
      </header>
      <div className="flex-1 min-h-0 overflow-hidden">{children}</div>
    </div>
  );
}
