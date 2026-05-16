import { NotificationsPanel } from "../components/NotificationsPanel";
import { SettingsLayout } from "../components/SettingsLayout";

export function SettingsNotifications() {
  return (
    <SettingsLayout title="Notifications" subtitle="Configure global notification rules">
      <div className="p-8 overflow-y-auto h-full">
        <div className="max-w-2xl">
          <NotificationsPanel appName="__global__" />
        </div>
      </div>
    </SettingsLayout>
  );
}
