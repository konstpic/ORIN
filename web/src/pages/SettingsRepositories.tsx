import { RepositoriesPage } from "./RepositoriesPage";
import { SettingsLayout } from "../components/SettingsLayout";

export function SettingsRepositories() {
  return (
    <SettingsLayout title="Settings" subtitle="Manage connected Git repositories">
      <RepositoriesPage />
    </SettingsLayout>
  );
}
