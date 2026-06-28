import * as Tabs from "@radix-ui/react-tabs";
import { RolesEditor } from "./RolesEditor";
import { ProjectsEditor } from "./ProjectsEditor";
import { BackendsEditor } from "./BackendsEditor";

export function SettingsPage() {
  return (
    <div className="settings-page">
      <h1>Settings</h1>
      <Tabs.Root defaultValue="roles" className="settings-tabs">
        <Tabs.List className="settings-tabs-list">
          <Tabs.Trigger value="roles">Roles</Tabs.Trigger>
          <Tabs.Trigger value="projects">Projects</Tabs.Trigger>
          <Tabs.Trigger value="backends">Backends</Tabs.Trigger>
        </Tabs.List>
        <Tabs.Content value="roles" className="settings-tab-content">
          <RolesEditor />
        </Tabs.Content>
        <Tabs.Content value="projects" className="settings-tab-content">
          <ProjectsEditor />
        </Tabs.Content>
        <Tabs.Content value="backends" className="settings-tab-content">
          <BackendsEditor />
        </Tabs.Content>
      </Tabs.Root>
    </div>
  );
}
