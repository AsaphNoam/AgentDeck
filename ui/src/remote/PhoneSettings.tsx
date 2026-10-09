import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { phoneFetch, unpairSelf } from "./api";
import { disconnect, rememberPaired, useConnection } from "./connection";
import { ConfirmDialog } from "../components/ui";
import { PhoneIcon } from "./PhoneIcon";

interface Self {
  id: string;
  name: string;
  notifications: "on" | "off" | "expired";
  vapid_public_key: string;
}

const errorText = (error: unknown) => (error instanceof Error ? error.message : String(error));

function keyBytes(base64url: string) {
  const padded = (base64url + "===".slice((base64url.length + 3) % 4)).replace(/-/g, "+").replace(/_/g, "/");
  const raw = atob(padded);
  const bytes = new Uint8Array(new ArrayBuffer(raw.length));
  for (let i = 0; i < raw.length; i++) bytes[i] = raw.charCodeAt(i);
  return bytes;
}

const pushSupported = () => "serviceWorker" in navigator && "PushManager" in window && "Notification" in window;
const iosBrowserTab = () =>
  /iPhone|iPad|iPod/.test(navigator.userAgent) && (navigator as Navigator & { standalone?: boolean }).standalone !== true;

/** "This phone": its name, attention notifications, and unpairing
 *  (FS-20.R8, R18, R28). */
export function PhoneSettings() {
  const link = useConnection((state) => state.link);
  const offline = link !== "connected";
  const client = useQueryClient();
  const self = useQuery({ queryKey: ["self"], queryFn: () => phoneFetch<Self>("/api/remote/self") });
  const [name, setName] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [unpairOpen, setUnpairOpen] = useState(false);

  const run = async (fn: () => Promise<unknown>) => {
    setBusy(true);
    setError(null);
    try {
      await fn();
    } catch (err) {
      setError(errorText(err));
    } finally {
      setBusy(false);
      void client.invalidateQueries({ queryKey: ["self"] });
    }
  };

  const enable = () =>
    run(async () => {
      if ((await Notification.requestPermission()) !== "granted") throw new Error("Notifications are blocked for Chuck in this phone's settings.");
      const registration = await navigator.serviceWorker.ready;
      const existing = await registration.pushManager.getSubscription();
      await existing?.unsubscribe();
      const subscription = await registration.pushManager.subscribe({
        userVisibleOnly: true,
        applicationServerKey: keyBytes(self.data!.vapid_public_key),
      });
      await phoneFetch("/api/remote/self/push", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(subscription.toJSON()),
      });
    });
  const disable = () =>
    run(async () => {
      const registration = await navigator.serviceWorker.ready;
      await (await registration.pushManager.getSubscription())?.unsubscribe();
      await phoneFetch("/api/remote/self/push", { method: "DELETE" });
    });

  const s = self.data;
  if (!s) return <p className="phone-empty">{self.isError ? errorText(self.error) : "Loading…"}</p>;
  const withdrawn = pushSupported() && Notification.permission === "denied";
  const lapsed = s.notifications === "expired" || (s.notifications === "on" && withdrawn);
  const connection = link === "connected" ? "Connected" : link === "unreachable" ? "Mac unreachable" : "Reconnecting";

  return (
    <div className="phone-agent">
      <header className="phone-page-heading">
        <span className="phone-eyebrow">YOUR COMPANION</span>
        <h1>This phone</h1>
        <p>Stay connected. On your terms.</p>
      </header>
      <section className="phone-phone-art" aria-label="Pairing status">
        <PhoneIcon name="phone" size={34} />
        <div>
          <strong>{s.name}</strong>
          <span>Paired with your Mac</span>
          <span className="phone-status" data-tone={offline ? "waiting" : "connected"}>
            <i aria-hidden="true" />{connection}
          </span>
        </div>
      </section>
      <details className="phone-details"><summary>Phone name</summary><form
        className="phone-settings-card phone-form"
        aria-label="Phone name"
        onSubmit={(event) => {
          event.preventDefault();
          if (name === null) return;
          void run(async () => {
            await phoneFetch("/api/remote/self", {
              method: "PATCH",
              headers: { "Content-Type": "application/json" },
              body: JSON.stringify({ name }),
            });
            setName(null);
          });
        }}
      >
        <div className="phone-section-title"><h2>Phone name</h2></div>
        <p className="phone-help">This is how your phone appears in Chuck’s Remote settings.</p>
        <label className="phone-field">
          Name for this phone
          <input maxLength={64} value={name ?? s.name} onChange={(event) => setName(event.target.value)} />
        </label>
        {name !== null && (
          <button type="submit" disabled={offline || busy || !name.trim()}>
            Save name
          </button>
        )}
      </form></details>
      <section className="phone-section" aria-label="Notifications">
        <div className="phone-section-title"><h2>Notifications</h2></div>
        <div className="phone-settings-card"><PhoneIcon name="bell" size={19} /><strong>Know when you’re needed</strong>
        <p className="phone-help">Know when an agent needs permission or a reply, hits an error, or a pipeline run needs you. Finished work never notifies.</p>
        {!pushSupported() || iosBrowserTab() ? (
          <p className="phone-help">Add Chuck to the Home Screen and open it from there to turn on notifications.</p>
        ) : lapsed ? (
          <>
            <p className="phone-error">Notifications are off: this phone stopped accepting them.</p>
            <button type="button" className="phone-primary" disabled={offline || busy} onClick={() => void enable()}>
              Turn notifications back on
            </button>
          </>
        ) : s.notifications === "on" ? (
          <button type="button" disabled={offline || busy} onClick={() => void disable()}>
            Turn notifications off
          </button>
        ) : (
          <button type="button" className="phone-primary" disabled={offline || busy} onClick={() => void enable()}>
            Turn notifications on
          </button>
        )}
        </div>
      </section>
      <section className="phone-section" aria-label="Connection">
        <div className="phone-section-title"><h2>Connection</h2></div><div className="phone-settings-card">
        <div className="phone-keyvalue"><span>Desktop</span><strong>Your Mac</strong></div>
        <div className="phone-keyvalue"><span>Status</span><strong>{connection}</strong></div>
        <p className="phone-help">Your Mac runs the work. This phone is a remote companion.</p>
        {offline && <p className="phone-help">The last known state is shown. Actions are unavailable until your Mac reconnects.</p>}
        </div>
      </section>
      {error && <p className="phone-error" role="alert">{error}</p>}
      <button
        type="button"
        className="phone-danger"
        disabled={offline || busy}
        onClick={() => setUnpairOpen(true)}
      >
        Unpair this phone
      </button>
      <p className="phone-help">Unpairing revokes this phone’s access. Your agents keep running on your Mac.</p>
      <div className="phone-help phone-desktop-note"><PhoneIcon name="shield" size={18} /><p>Projects, roles, runtimes, and templates are managed on your Mac.</p></div>
      <ConfirmDialog
        open={unpairOpen}
        title="Unpair this phone?"
        confirmLabel="Unpair phone"
        destructive
        pending={busy}
        confirmDisabled={offline}
        onCancel={() => setUnpairOpen(false)}
        onConfirm={() => void run(async () => {
          await unpairSelf();
          rememberPaired(false);
          disconnect();
          useConnection.getState().setLink("unpaired");
          setUnpairOpen(false);
        })}
      >
        <p>This phone will lose access to Chuck and will need to be paired again from the Mac.</p>
        <p>Your agents keep running on your Mac.</p>
        {error && <p className="phone-error" role="alert">{error}</p>}
      </ConfirmDialog>
    </div>
  );
}
