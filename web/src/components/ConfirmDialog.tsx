import { useEffect, useRef, useState } from "react";
import { AlertTriangle } from "lucide-react";

export function ConfirmDialog({
  title,
  description,
  confirmLabel = "Confirm",
  danger = false,
  onConfirm,
  onCancel,
}: {
  title: string;
  description?: string;
  confirmLabel?: string;
  danger?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  const cancelRef = useRef<HTMLButtonElement>(null);
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    cancelRef.current?.focus();
    const key = (e: KeyboardEvent) => {
      if (e.key === "Escape") onCancel();
    };
    document.addEventListener("keydown", key);
    // Trigger entrance animation
    requestAnimationFrame(() => setMounted(true));
    return () => document.removeEventListener("keydown", key);
  }, [onCancel]);

  return (
    <div
      className="fixed inset-0 z-[10000] flex items-center justify-center transition-opacity duration-200"
      style={{ opacity: mounted ? 1 : 0 }}
    >
      {/* Backdrop */}
      <div
        className="absolute inset-0 bg-black/50 backdrop-blur-sm"
        onClick={onCancel}
      />
      {/* Dialog */}
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="confirm-title"
        className="relative w-full max-w-sm mx-4 rounded-xl border border-[var(--color-border-strong)] bg-[var(--color-elevated)] shadow-2xl p-5 transition-all duration-200 ease-out"
        style={{ transform: mounted ? "scale(1)" : "scale(0.95)", opacity: mounted ? 1 : 0 }}
      >
        <div className="flex items-start gap-3 mb-4">
          {danger && (
            <span className="mt-0.5 shrink-0 inline-flex items-center justify-center size-8 rounded-full bg-red-500/15 text-red-400">
              <AlertTriangle className="size-4" strokeWidth={2} />
            </span>
          )}
          <div>
            <h2 id="confirm-title" className="text-sm font-semibold text-[var(--color-text)]">
              {title}
            </h2>
            {description && (
              <p className="mt-1 text-xs text-[var(--color-text-muted)]">{description}</p>
            )}
          </div>
        </div>
        <div className="flex justify-end gap-2">
          <button
            ref={cancelRef}
            type="button"
            onClick={onCancel}
            className="rounded-md px-3 py-1.5 text-xs font-medium border border-[var(--color-border)] text-[var(--color-text-muted)] hover:text-[var(--color-text)] hover:border-[var(--color-border-strong)] transition-all duration-150 hover:scale-[1.02] active:scale-[0.98]"
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={onConfirm}
            className={`rounded-md px-3 py-1.5 text-xs font-medium transition-all duration-150 hover:scale-[1.02] active:scale-[0.98] ${
              danger
                ? "bg-red-600 hover:bg-red-500 text-white"
                : "bg-[var(--color-accent)] hover:opacity-90 text-white"
            }`}
          >
            {confirmLabel}
          </button>
        </div>
      </div>
    </div>
  );
}
