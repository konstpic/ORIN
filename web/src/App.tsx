import { Navigate, Route, Routes } from "react-router-dom";
import { Shell } from "./components/Shell";
import { LoginPage } from "./pages/LoginPage";
import { AppsPage } from "./pages/AppsPage";
import { AppDetailPage } from "./pages/AppDetailPage";
import { SettingsRepositories } from "./pages/SettingsRepositories";
import { SettingsClusters } from "./pages/SettingsClusters";
import { SettingsNotifications } from "./pages/SettingsNotifications";
import { SettingsSystem } from "./pages/SettingsSystem";
import { SettingsRBAC } from "./pages/SettingsRBAC";
import { useAuth } from "./state/auth";

export default function App() {
  const token = useAuth((s) => s.token);
  if (!token) {
    return (
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="*" element={<Navigate to="/login" replace />} />
      </Routes>
    );
  }
  return (
    <Shell>
      <Routes>
        <Route path="/" element={<Navigate to="/applications" replace />} />
        <Route path="/applications" element={<AppsPage />} />
        <Route path="/applications/:name/*" element={<AppDetailPage />} />
        <Route path="/settings/repositories" element={<SettingsRepositories />} />
        <Route path="/settings/clusters" element={<SettingsClusters />} />
        <Route path="/settings/notifications" element={<SettingsNotifications />} />
        <Route path="/settings/rbac" element={<SettingsRBAC />} />
        <Route path="/settings/system" element={<SettingsSystem />} />
        <Route path="/settings/*" element={<Navigate to="/settings/repositories" replace />} />
        <Route path="*" element={<Navigate to="/applications" replace />} />
      </Routes>
    </Shell>
  );
}
