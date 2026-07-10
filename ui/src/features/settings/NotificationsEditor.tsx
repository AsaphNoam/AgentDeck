import { useConfig, usePutConfig } from "../../api/config";
import { useUiStore } from "../../store/uiStore";
import type { NotificationType } from "../../api/types";

const notificationTypes: Array<{ type: NotificationType; label: string }> = [
  { type: "done", label: "Done" },
  { type: "waiting_input", label: "Needs input" },
  { type: "permission_required", label: "Permission" },
  { type: "budget_exceeded", label: "Budget" },
];

export function NotificationsEditor() {
  const { data: config } = useConfig();
  const putConfig = usePutConfig();
  const pushError = useUiStore((state) => state.pushError);
  const notifications = config?.notifications ?? {
    desktop_enabled: true,
    muted: { done: false, waiting_input: false, permission_required: false, budget_exceeded: false },
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
    <div className="config-editor notifications-editor">
      <div className="config-editor-header">
        <h2>Notifications</h2>
        {"Notification" in window && Notification.permission !== "granted" && (
          <button type="button" onClick={() => void requestDesktop()}>
            Enable desktop
          </button>
        )}
      </div>
      <label className="toggle-row">
        <input
          type="checkbox"
          checked={notifications.desktop_enabled}
          onChange={(event) => save({ ...notifications, desktop_enabled: event.target.checked })}
        />
        Desktop notifications
      </label>
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
