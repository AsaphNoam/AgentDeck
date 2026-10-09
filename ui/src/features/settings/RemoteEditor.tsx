import { useEffect, useState } from "react";
import {
  useCreatePairing,
  usePairingDecision,
  usePutRemote,
  useRemote,
  useRenameDevice,
  useRevokeDevice,
  type RemoteDevice,
  type RemotePairingCode,
  type RemoteStatus,
} from "../../api/remote";
import { useUiStore } from "../../store/uiStore";
import { SettingsHeader } from "./SettingsHeader";

const errorText = (error: unknown) => (error instanceof Error ? error.message : String(error));

// Unavailable reasons map to a stated repair (FS-20.R2).
const unavailableRepair: Record<string, string> = {
  magicdns_disabled: "Turn on MagicDNS in the DNS page of the Tailscale admin console.",
  https_disabled: "Turn on HTTPS certificates in the DNS page of the Tailscale admin console.",
  node_error: "Chuck could not start its Tailscale device. Turn remote control off and on again.",
  listener_error: "Chuck could not open the phone address. Turn remote control off and on again.",
};

function StatusLine({ status }: { status: RemoteStatus }) {
  switch (status.state) {
    case "off":
      return <p className="remote-status">Off</p>;
    case "starting":
      return <p className="remote-status">Connecting…</p>;
    case "needs_login":
      return (
        <div className="remote-status">
          <p>Needs Tailscale sign-in. Approve this Mac as a device on your tailnet.</p>
          {status.auth_url ? (
            <a href={status.auth_url} target="_blank" rel="noreferrer">
              Sign in to Tailscale
            </a>
          ) : (
            <p className="config-notice">Waiting for a sign-in link…</p>
          )}
        </div>
      );
    case "on":
      return (
        <p className="remote-status">
          On — phone address <code>{status.address}</code>
        </p>
      );
    default:
      return (
        <div className="remote-status">
          <p>Unavailable</p>
          <p className="form-error">{unavailableRepair[status.reason ?? ""] ?? "Remote control is unavailable."}</p>
        </div>
      );
  }
}

function PairingPanel({ status }: { status: RemoteStatus }) {
  const createPairing = useCreatePairing();
  const decide = usePairingDecision();
  const pushError = useUiStore((state) => state.pushError);
  const [code, setCode] = useState<RemotePairingCode | null>(null);
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    if (!code) return;
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [code]);

  const pending = status.pending_pairing;
  // A claimed code is spent; the request replaces it on screen.
  useEffect(() => {
    if (pending) setCode(null);
  }, [pending]);

  const expired = code !== null && new Date(code.expires_at).getTime() <= now;

  if (pending) {
    return (
      <div className="remote-pairing" data-slot="form" role="group" aria-label="Pairing request">
        <h3>Allow this phone?</h3>
        <p>
          <strong>{pending.name}</strong>
          {pending.login ? ` · ${pending.login}` : ""}
        </p>
        <div className="remote-actions" data-slot="actions">
          <button
            type="button"
            disabled={decide.isPending}
            onClick={() =>
              decide.mutate({ id: pending.id, allow: true }, { onError: (e) => pushError("Allowing the phone failed", errorText(e)) })
            }
          >
            Allow
          </button>
          <button
            type="button"
            disabled={decide.isPending}
            onClick={() =>
              decide.mutate({ id: pending.id, allow: false }, { onError: (e) => pushError("Declining the phone failed", errorText(e)) })
            }
          >
            Decline
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="remote-pairing" data-slot="form">
      <p className="config-notice">
        Your phone needs the Tailscale app, signed in to the same tailnet. Chuck does not install or configure it.
      </p>
      {code && !expired ? (
        <div className="remote-code">
          <img className="remote-qr" alt="Pairing QR code" src={`data:image/svg+xml;utf8,${encodeURIComponent(code.qr_svg)}`} />
          <div>
            <p>Scan with the phone camera, or open the phone address and enter:</p>
            <p className="remote-code-value" aria-label="Pairing code">
              {code.code}
            </p>
            <p className="config-notice">
              Valid once, for {Math.max(0, Math.ceil((new Date(code.expires_at).getTime() - now) / 60000))} more minute(s). On
              iPhone, add the app to the Home Screen first and enter the code there.
            </p>
          </div>
        </div>
      ) : (
        expired && <p className="form-error">That code expired. Show a new one.</p>
      )}
      <button
        type="button"
        disabled={createPairing.isPending}
        onClick={() =>
          createPairing.mutate(undefined, {
            onSuccess: (next) => {
              setCode(next);
              setNow(Date.now());
            },
            onError: (e) => pushError("Pairing failed", errorText(e)),
          })
        }
      >
        {code ? "Show a new code" : "Pair a phone"}
      </button>
    </div>
  );
}

function DeviceRow({ device }: { device: RemoteDevice }) {
  const rename = useRenameDevice();
  const revoke = useRevokeDevice();
  const pushError = useUiStore((state) => state.pushError);
  const [draft, setDraft] = useState<string | null>(null);
  const date = (value: string) => new Date(value).toLocaleString();

  return (
    <li className="config-list-item" data-slot="item">
      {draft === null ? (
        <div>
          <strong>{device.name}</strong>
          <p className="remote-device-meta">
            Paired {date(device.paired_at)} · Last seen {date(device.last_seen_at)} · Notifications{" "}
            {device.notifications === "expired" ? "off (subscription expired)" : device.notifications}
          </p>
        </div>
      ) : (
        <form
          onSubmit={(event) => {
            event.preventDefault();
            rename.mutate(
              { id: device.id, name: draft },
              { onSuccess: () => setDraft(null), onError: (e) => pushError("Renaming the phone failed", errorText(e)) },
            );
          }}
        >
          <label className="form-field">
            Phone name
            <input aria-label="Phone name" maxLength={64} value={draft} onChange={(event) => setDraft(event.target.value)} />
          </label>
          <button type="submit" disabled={!draft.trim() || rename.isPending}>
            Save
          </button>
          <button type="button" onClick={() => setDraft(null)}>
            Cancel
          </button>
        </form>
      )}
      {draft === null && (
        <div className="remote-actions" data-slot="actions">
          <button type="button" className="config-text-action" onClick={() => setDraft(device.name)}>
            Rename
          </button>
          <button
            type="button"
            className="config-text-action config-text-action-danger"
            disabled={revoke.isPending}
            onClick={() => {
              if (!window.confirm(`Revoke ${device.name}? It loses access immediately and must pair again.`)) return;
              revoke.mutate(device.id, { onError: (e) => pushError("Revoking the phone failed", errorText(e)) });
            }}
          >
            Revoke
          </button>
        </div>
      )}
    </li>
  );
}

export function RemoteEditor() {
  const remote = useRemote();
  const putRemote = usePutRemote();
  const pushError = useUiStore((state) => state.pushError);
  const status = remote.data;

  const save = (body: { enabled?: boolean; keep_awake?: boolean }, what: string) =>
    putRemote.mutate(body, { onError: (e) => pushError(`Saving ${what} failed`, errorText(e)) });

  return (
    <div className="config-editor" data-ui="config-editor" data-variant="remote">
      <SettingsHeader eyebrow="Remote" title="Remote" description="Supervise and direct Chuck from your phone while this Mac keeps working." />
      {remote.isError && <p className="form-error">{errorText(remote.error)}</p>}
      {status && (
        <>
          <p className="config-notice">
            Remote control joins your Tailscale network as a device named <code>chuck</code>, reachable only from
            your tailnet. It needs MagicDNS and HTTPS certificates; turning on HTTPS certificates publishes the name{" "}
            <code>chuck.&lt;tailnet&gt;.ts.net</code> in public certificate-transparency logs.
          </p>
          <label className="toggle-row">
            <input
              type="checkbox"
              checked={status.state !== "off"}
              disabled={putRemote.isPending}
              onChange={(event) => save({ enabled: event.target.checked }, "remote control")}
            />
            Remote control
          </label>
          <StatusLine status={status} />
          {status.state === "on" && <PairingPanel status={status} />}
          <h3 className="config-group-label">Paired phones</h3>
          {status.devices.length === 0 ? (
            <p className="config-empty" data-state="empty">
              No phones are paired.
            </p>
          ) : (
            <ul className="config-list" data-slot="list">
              {status.devices.map((device) => (
                <DeviceRow key={device.id} device={device} />
              ))}
            </ul>
          )}
          <label className="toggle-row">
            <input
              type="checkbox"
              checked={status.keep_awake}
              disabled={!status.keep_awake_available || putRemote.isPending}
              onChange={(event) => save({ keep_awake: event.target.checked }, "keep awake")}
            />
            Keep this Mac awake while work is active
          </label>
          <p className="config-notice">
            {status.keep_awake_available
              ? "Prevents idle sleep while an agent is busy, a permission is waiting, or a pipeline run is active. Closing the lid or choosing Sleep still sleeps the Mac."
              : "Keeping the computer awake is available only on macOS."}
          </p>
        </>
      )}
    </div>
  );
}
