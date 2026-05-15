import { DiffView } from "./DiffView";

export function DiffDrawer({
  appName,
  open,
  onClose,
}: {
  appName: string;
  open: boolean;
  onClose: () => void;
}) {
  if (!open) return null;
  return (
    <div className="fixed inset-0 z-50 flex justify-end pointer-events-none">
      <button type="button" className="absolute inset-0 bg-black/50 pointer-events-auto" aria-label="Close diff" onClick={onClose} />
      <aside className="relative pointer-events-auto h-full w-full max-w-[min(1180px,100vw)] border-l border-[var(--color-border)] bg-[var(--color-surface)] shadow-2xl flex flex-col">
        <div className="shrink-0 px-4 py-3 border-b border-[var(--color-border)] bg-[var(--color-surface-muted)] flex items-center justify-between gap-3">
          <div>
            <h2 className="text-sm font-semibold uppercase tracking-wide text-[var(--color-text)]">Diff</h2>
            <p className="text-xs text-[var(--color-text-muted)] mt-0.5">Live vs desired · all resources</p>
          </div>
          <button type="button" className="text-xs text-[var(--color-text-muted)] hover:text-[var(--color-accent)] underline" onClick={onClose}>
            Close
          </button>
        </div>
        <div className="flex-1 min-h-0 overflow-hidden flex flex-col p-3">
          <DiffView name={appName} variant="drawer" />
        </div>
      </aside>
    </div>
  );
}
