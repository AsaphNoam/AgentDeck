import * as Tabs from "@radix-ui/react-tabs";
import { RolesEditor } from "./RolesEditor";
import { ProjectsEditor } from "./ProjectsEditor";
import { BackendsEditor } from "./BackendsEditor";
import { NotificationsEditor } from "./NotificationsEditor";

export function SettingsPage() {
  return (
    <div className="settings-page" data-ui="settings">
      <h1 data-slot="header">Settings</h1>
      <Tabs.Root defaultValue="roles" className="settings-tabs">
        <Tabs.List className="settings-tabs-list" data-slot="navigation">
          <Tabs.Trigger value="roles">Roles</Tabs.Trigger>
          <Tabs.Trigger value="projects">Projects</Tabs.Trigger>
          <Tabs.Trigger value="backends">Backends</Tabs.Trigger>
          <Tabs.Trigger value="notifications">Notifications</Tabs.Trigger>
        </Tabs.List>
        <Tabs.Content value="roles" className="settings-tab-content" data-slot="content">
          <RolesEditor />
        </Tabs.Content>
        <Tabs.Content value="projects" className="settings-tab-content" data-slot="content">
          <ProjectsEditor />
        </Tabs.Content>
        <Tabs.Content value="backends" className="settings-tab-content" data-slot="content">
          <BackendsEditor />
        </Tabs.Content>
        <Tabs.Content value="notifications" className="settings-tab-content" data-slot="content">
          <NotificationsEditor />
        </Tabs.Content>
      </Tabs.Root>
    </div>
  );
}
