import { SettingsLayout } from "../components/SettingsLayout";

export function SettingsSystem() {
  return (
    <SettingsLayout title="System" subtitle="System configuration and maintenance settings">
      <div className="p-8">
        <div className="text-sm text-[var(--color-text-muted)]">
          System configuration — coming soon. This will include reconciliation settings, sync windows, and maintenance mode.
        </div>
      </div>
    </SettingsLayout>
  );
}
