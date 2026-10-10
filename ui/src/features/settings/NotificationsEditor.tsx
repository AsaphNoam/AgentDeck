import { useConfig, usePutConfig } from "../../api/config";
import { useUiStore } from "../../store/uiStore";
import type { NotificationType } from "../../api/types";
import { SettingsHeader } from "./SettingsHeader";

const notificationTypes: Array<{ type: NotificationType; label: string }> = [
  { type: "done", label: "Done" },
  { type: "waiting_input", label: "Needs input" },
  { type: "permission_required", label: "Permission" },
  { type: "budget_exceeded", label: "Budget" },
  { type: "pipeline_needs_attention", label: "Pipeline needs attention" },
  { type: "pipeline_completed", label: "Pipeline completed" },
];

export function NotificationsEditor() {
  const { data: config } = useConfig();
  const putConfig = usePutConfig();
  const pushError = useUiStore((state) => state.pushError);
  const notifications = config?.notifications ?? {
    desktop_enabled: true,
    muted: { done: false, waiting_input: false, permission_required: false, budget_exceeded: false, pipeline_needs_attention: false, pipeline_completed: false },
  };

  const save = (next: typeof notifications) => {
    putConfig.mutate(
      { notifications: next },
      {
        onError: (err: unknown) =>
          pushError("Saving notifications failed", err instanceof Error ? err.message : String(err)),
      },
    );
  };
  const requestDesktop = async () => {
    if (!("Notification" in window)) return;
    const permission = await Notification.requestPermission();
    if (permission === "granted") save({ ...notifications, desktop_enabled: true });
  };

  return (
    <div className="config-editor notifications-editor" data-ui="config-editor" data-variant="notifications">
      <SettingsHeader eyebrow="Notifications" title="Notifications" />
      <div className="notification-desktop">
        <label className="toggle-row toggle-row-primary">
          <input
            type="checkbox"
            checked={notifications.desktop_enabled}
            onChange={(event) => save({ ...notifications, desktop_enabled: event.target.checked })}
          />
          Desktop notifications
        </label>
        {"Notification" in window && Notification.permission !== "granted" && (
          <button type="button" className="ad-button-secondary" onClick={() => void requestDesktop()}>
            Enable desktop
          </button>
        )}
      </div>
      <div className="notification-mutes">
        {notificationTypes.map((item) => (
          <label key={item.type} className="toggle-row">
            <input
              type="checkbox"
              checked={!notifications.muted[item.type]}
              onChange={(event) =>
                save({
                  ...notifications,
                  muted: { ...notifications.muted, [item.type]: !event.target.checked },
                })
              }
            />
            {item.label}
          </label>
        ))}
      </div>
    </div>
  );
}
