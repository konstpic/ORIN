import { Navigate, Route, Routes } from "react-router-dom";
import { Shell } from "./components/Shell";
import { LoginPage } from "./pages/LoginPage";
import { AppsPage } from "./pages/AppsPage";
import { AppDetailPage } from "./pages/AppDetailPage";
import { RepositoriesPage } from "./pages/RepositoriesPage";
import { ClustersPage } from "./pages/ClustersPage";
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
        <Route path="/settings/repositories" element={<RepositoriesPage />} />
        <Route path="/settings/clusters" element={<ClustersPage />} />
        <Route path="*" element={<Navigate to="/applications" replace />} />
      </Routes>
    </Shell>
  );
}
