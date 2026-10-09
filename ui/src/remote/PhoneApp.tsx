import { useEffect, useState, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import * as Dialog from "@radix-ui/react-dialog";
import { applyAppearance } from "../features/appearance/appearance";
import { ChuckMark } from "../components/shell/ChuckMark";
import { phoneFetch } from "./api";
import { PhoneIcon } from "./PhoneIcon";
import { classify, connect, useConnection, wasPaired } from "./connection";
import { HomeScreen } from "./HomeScreen";
import { PairScreen, UnpairedScreen } from "./PairScreen";
import { match, navigate, usePath } from "./router";
import { AgentScreen } from "./AgentScreen";
import { PhoneSettings } from "./PhoneSettings";
import { RunScreen } from "./RunScreen";
import { ProjectScreen } from "./ProjectScreen";

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
  const [menuOpen, setMenuOpen] = useState(false);
  const path = usePath();
  const config = useQuery({ queryKey: ["phone-appearance"], queryFn: () => phoneFetch<{ appearance_skin?: string }>("/api/config"), refetchInterval: 5000, enabled: link === "connected" });
  const skin = config.isError ? undefined : config.data?.appearance_skin;
  useEffect(() => { applyAppearance(skin); }, [skin]);
  const go = (route: string) => { setMenuOpen(false); navigate(route); };
  return (
    <div className="phone-app">
      <header className="phone-header">
        <Dialog.Root open={menuOpen} onOpenChange={setMenuOpen}>
          <Dialog.Trigger className="phone-menu-trigger" aria-label="Open navigation"><PhoneIcon name="menu" size={21} /></Dialog.Trigger>
          <Dialog.Portal>
            <Dialog.Overlay className="phone-navigation-overlay" />
            <Dialog.Content className="phone-navigation" aria-describedby={undefined}>
              <Dialog.Title className="phone-eyebrow">Your companion</Dialog.Title>
              <nav aria-label="Mobile navigation">
                <button type="button" aria-current={path === "/" ? "page" : undefined} onClick={() => go("/")}><PhoneIcon name="home" />Home<PhoneIcon name="arrow" size={16} /></button>
                <button type="button" aria-current={path === "/phone" ? "page" : undefined} onClick={() => go("/phone")}><PhoneIcon name="phone" />This phone<PhoneIcon name="arrow" size={16} /></button>
              </nav>
            </Dialog.Content>
          </Dialog.Portal>
        </Dialog.Root>
        <button type="button" className="phone-brand" aria-label="Chuck Home" onClick={() => navigate("/")}><ChuckMark compact /><strong>Chuck</strong><span className="phone-companion">companion</span></button>
      </header>
      <Banner />
      <main className="phone-screen" aria-busy={link === "checking"} data-stale={link === "unreachable" ? "true" : undefined}>
        {back && !match(path, "agent") && !match(path, "run") && <button type="button" className="phone-back" onClick={() => navigate("/")}><PhoneIcon name="back" size={16} />Home</button>}
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
    if (link === "unpaired") applyAppearance();
  }, [link]);

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
  if (agentId) return <AgentScreen key={agentId} agentId={agentId} />;
  const projectId = match(path, "project");
  if (projectId) return <ProjectScreen key={projectId} projectID={projectId} />;
  const runId = match(path, "run");
  if (runId) return <RunScreen runId={runId} />;
  if (path === "/phone") return <PhoneSettings />;
  return <HomeScreen />;
}
