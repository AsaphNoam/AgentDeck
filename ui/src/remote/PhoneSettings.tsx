import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { phoneFetch, unpairSelf } from "./api";
import { disconnect, rememberPaired, useConnection } from "./connection";

interface Self {
  id: string;
  name: string;
  notifications: "on" | "off" | "expired";
  vapid_public_key: string;
}

const errorText = (error: unknown) => (error instanceof Error ? error.message : String(error));

function keyBytes(base64url: string): Uint8Array {
  const padded = (base64url + "===".slice((base64url.length + 3) % 4)).replace(/-/g, "+").replace(/_/g, "/");
  return Uint8Array.from(atob(padded), (c) => c.charCodeAt(0));
}

const pushSupported = () => "serviceWorker" in navigator && "PushManager" in window && "Notification" in window;
const iosBrowserTab = () =>
  /iPhone|iPad|iPod/.test(navigator.userAgent) && (navigator as Navigator & { standalone?: boolean }).standalone !== true;

/** "This phone": its name, attention notifications, and unpairing
 *  (FS-20.R8, R18, R28). */
export function PhoneSettings() {
  const offline = useConnection((state) => state.link !== "connected");
  const client = useQueryClient();
  const self = useQuery({ queryKey: ["self"], queryFn: () => phoneFetch<Self>("/api/remote/self") });
  const [name, setName] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

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
      if ((await Notification.requestPermission()) !== "granted") throw new Error("Notifications are blocked for AgentDeck in this phone's settings.");
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

  return (
    <div className="phone-agent">
      <h1>This phone</h1>
      <form
        className="phone-card phone-form"
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
        <label className="phone-field">
          Name
          <input maxLength={64} value={name ?? s.name} onChange={(event) => setName(event.target.value)} />
        </label>
        {name !== null && (
          <button type="submit" disabled={offline || busy || !name.trim()}>
            Save name
          </button>
        )}
      </form>
      <section className="phone-card" aria-label="Notifications">
        <p className="phone-card-kicker">Notifications</p>
        <p>Get notified when an agent needs permission or a reply, hits an error, or a task or run needs you. Finished work never notifies.</p>
        {!pushSupported() || iosBrowserTab() ? (
          <p className="phone-meta">Add AgentDeck to the Home Screen and open it from there to turn on notifications.</p>
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
      </section>
      {error && <p className="phone-error">{error}</p>}
      <button
        type="button"
        className="phone-danger"
        disabled={offline || busy}
        onClick={() => {
          if (!window.confirm("Unpair this phone? It will need to be paired again from the Mac.")) return;
          void run(async () => {
            await unpairSelf();
            rememberPaired(false);
            disconnect();
            useConnection.getState().setLink("unpaired");
          });
        }}
      >
        Unpair this phone
      </button>
    </div>
  );
}
