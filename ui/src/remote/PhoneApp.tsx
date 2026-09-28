import { useEffect, type ReactNode } from "react";
import { classify, connect, useConnection, wasPaired } from "./connection";
import { HomeScreen } from "./HomeScreen";
import { PairScreen, UnpairedScreen } from "./PairScreen";
import { match, navigate, usePath } from "./router";
import { AgentScreen } from "./AgentScreen";
import { NewWorkScreen } from "./NewWorkScreen";
import { PhoneSettings } from "./PhoneSettings";
import { RunScreen, TaskScreen } from "./WorkScreens";

function Banner() {
  const link = useConnection((state) => state.link);
  const since = useConnection((state) => state.unreachableSince);
  if (link === "reconnecting") return <p className="phone-banner" role="status">Reconnecting…</p>;
  if (link !== "unreachable") return null;
  const at = since ? new Date(since).toLocaleTimeString([], { hour: "numeric", minute: "2-digit" }) : "";
  return (
    <p className="phone-banner phone-banner-alert" role="alert">
      Mac unreachable since {at}. Showing the last known state; actions are off until it is back.
    </p>
  );
}

/** Shell is the paired app frame: header, connection banner, and content that
 *  is visibly stale while the Mac is unreachable (FS-20.R23). */
function Shell({ children, back }: { children: ReactNode; back?: boolean }) {
  const link = useConnection((state) => state.link);
  return (
    <div className="phone-app">
      <header className="phone-header">
        {back ? (
          <button type="button" className="phone-back" onClick={() => navigate("/")}>
            ‹ Home
          </button>
        ) : (
          <>
            <span className="phone-brand">AgentDeck</span>
            <span className="phone-header-actions">
              <button type="button" onClick={() => navigate("/new")}>
                New work
              </button>
              <button type="button" aria-label="This phone" onClick={() => navigate("/phone")}>
                ⚙
              </button>
            </span>
          </>
        )}
      </header>
      <Banner />
      <main className="phone-screen" aria-busy={link === "checking"} data-stale={link === "unreachable" ? "true" : undefined}>
        {children}
      </main>
    </div>
  );
}

export function PhoneApp() {
  const link = useConnection((state) => state.link);
  const setLink = useConnection((state) => state.setLink);
  const path = usePath();

  useEffect(() => {
    void classify()
      .then((result) => {
        if (result !== "unpaired") connect();
      })
      .catch(() => setLink("unreachable"));
  }, [setLink]);

  const paired = () => {
    setLink("checking");
    void classify().then((result) => result === "connected" && connect());
  };

  if (link === "unpaired" || path === "/pair") {
    if (wasPaired() && path !== "/pair") return <UnpairedScreen onPairAgain={() => navigate("/pair")} />;
    return <PairScreen onPaired={paired} />;
  }
  if (link === "checking") return <main className="phone-screen phone-center">Connecting to your Mac…</main>;
  return <Shell back={path !== "/"}>{screenFor(path)}</Shell>;
}

function screenFor(path: string) {
  const agentId = match(path, "agent");
  if (agentId) return <AgentScreen agentId={agentId} />;
  const taskId = match(path, "task");
  if (taskId) return <TaskScreen taskId={taskId} />;
  const runId = match(path, "run");
  if (runId) return <RunScreen runId={runId} />;
  if (path === "/new") return <NewWorkScreen />;
  if (path === "/phone") return <PhoneSettings />;
  return <HomeScreen />;
}
