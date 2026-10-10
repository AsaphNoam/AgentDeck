import { useEffect, useState } from "react";
import { claimPairing, PhoneAPIError, waitPairing } from "./api";
import { rememberPaired } from "./connection";
import { navigate } from "./router";
import { PhoneIcon } from "./PhoneIcon";

const isIOS = () => /iPhone|iPad|iPod/.test(navigator.userAgent);
const isStandalone = () =>
  (navigator as Navigator & { standalone?: boolean }).standalone === true ||
  window.matchMedia?.("(display-mode: standalone)").matches === true;

function proposedName() {
  if (isIOS()) return "iPhone";
  if (/Android/.test(navigator.userAgent)) return "Android phone";
  return "Phone";
}

const startAgain = "That code did not work. Show a new code in Chuck on your Mac and try again.";

export function PairScreen({ onPaired }: { onPaired: () => void }) {
  const [code, setCode] = useState(() => window.location.hash.replace(/^#/, "").toUpperCase());
  const [name, setName] = useState(proposedName);
  const [phase, setPhase] = useState<"form" | "waiting">("form");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    // The code rode in the fragment; drop it from the address bar.
    if (window.location.hash) window.history.replaceState(null, "", window.location.pathname);
  }, []);

  // On iPhone a Home Screen app has its own storage, so pairing must finish
  // inside the installed app (FS-20.R6).
  if (isIOS() && !isStandalone()) {
    return (
      <main className="phone-screen phone-center">
        <div className="phone-pair-icon"><PhoneIcon name="phone" size={42} /></div>
        <header className="phone-page-heading">
          <span className="phone-eyebrow">DESKTOP → PHONE</span>
          <h1>Add Chuck to your Home Screen</h1>
          <p>On iPhone, pairing has to happen inside the installed app so it can keep its own secure connection.</p>
        </header>
        <ol className="phone-steps">
          <li>Tap the Share button, then <strong>Add to Home Screen</strong>.</li>
          <li>Open Chuck from your Home Screen.</li>
          <li>Enter the pairing code there{code ? ":" : "."}</li>
        </ol>
        {code && <p className="phone-code" aria-label="Pairing code">{code}</p>}
      </main>
    );
  }

  const submit = async () => {
    setError(null);
    try {
      const { pending_id } = await claimPairing(code.trim(), name.trim());
      setPhase("waiting");
      for (;;) {
        const result = await waitPairing(pending_id);
        if (result.status === "allowed") break;
      }
      rememberPaired(true);
      navigate("/", true);
      onPaired();
    } catch (err) {
      setPhase("form");
      if (err instanceof PhoneAPIError && err.code === "remote_pairing_invalid") setError(startAgain);
      else setError(err instanceof Error ? err.message : String(err));
    }
  };

  if (phase === "waiting") {
    return (
      <main className="phone-screen phone-center" aria-live="polite">
        <div className="phone-pair-icon"><PhoneIcon name="phone" size={42} /></div>
        <header className="phone-page-heading">
          <span className="phone-eyebrow">DESKTOP → PHONE</span>
          <h1>Waiting for approval</h1>
          <p>Allow this phone in Chuck on your Mac to finish pairing.</p>
        </header>
        <section className="phone-settings-card">
          <span className="phone-status" data-tone="waiting"><i aria-hidden="true" />Waiting for desktop approval</span>
          <p>Chuck on your Mac is asking <strong>Allow this phone?</strong> for “{name.trim()}”. Keep this screen open.</p>
        </section>
      </main>
    );
  }

  return (
    <main className="phone-screen phone-center">
      <div className="phone-pair-icon"><PhoneIcon name="phone" size={42} /></div>
      <header className="phone-page-heading">
        <span className="phone-eyebrow">DESKTOP → PHONE</span>
        <h1>Pair this phone</h1>
        <p>Start pairing in Chuck on your Mac, then enter the one-time code here.</p>
      </header>
      <section className="phone-settings-card">
        <div className="phone-section-title"><PhoneIcon name="shield" size={19} /><h2>Private to your tailnet</h2></div>
        <p className="phone-help">Open Chuck → Settings → Remote → <strong>Pair a phone</strong>. This phone needs the Tailscale app signed in to the same tailnet.</p>
      </section>
      <form
        className="phone-form"
        onSubmit={(event) => {
          event.preventDefault();
          void submit();
        }}
      >
        <label className="phone-field">
          Pairing code
          <input
            autoCapitalize="characters"
            autoComplete="one-time-code"
            maxLength={8}
            value={code}
            onChange={(event) => setCode(event.target.value.toUpperCase())}
          />
        </label>
        <label className="phone-field">
          Name for this phone
          <input maxLength={64} value={name} onChange={(event) => setName(event.target.value)} />
        </label>
        {error && <p className="phone-error">{error}</p>}
        <button type="submit" className="ad-button-primary" disabled={code.trim().length !== 8 || !name.trim()}>
          Pair <PhoneIcon name="arrow" size={17} />
        </button>
      </form>
      <p className="phone-help">The code works once and expires after five minutes. No desktop password is entered on this phone.</p>
    </main>
  );
}

export function UnpairedScreen({ onPairAgain }: { onPairAgain: () => void }) {
  return (
    <main className="phone-screen phone-center">
      <h1>This phone was unpaired</h1>
      <p>It can no longer see or control Chuck. Pair it again from Chuck on your Mac.</p>
      <button
        type="button"
        className="ad-button-primary"
        onClick={() => {
          rememberPaired(false);
          onPairAgain();
        }}
      >
        Pair again
      </button>
    </main>
  );
}
