import { ClustersPage } from "./ClustersPage";
import { SettingsLayout } from "../components/SettingsLayout";

export function SettingsClusters() {
  return (
    <SettingsLayout title="Clusters" subtitle="Manage connected Kubernetes clusters">
      <ClustersPage />
    </SettingsLayout>
  );
}
