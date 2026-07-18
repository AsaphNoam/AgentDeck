import { useEffect, useRef } from "react";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import { resolvePresentationColors } from "../../presentation/resolveColors";
import { xtermTheme } from "../../presentation/integrations";

// TerminalTab renders a real xterm.js emulator bridged to the agent's PTY over a
// WebSocket (§3.4, task 13). The bridge contract: keystrokes go to the PTY as
// *binary* frames (text frames are reserved for resize), and a {cols,rows} text
// frame keeps the PTY window in step with the visible terminal.
export function TerminalTab({ agentId }: { agentId: string }) {
  const containerRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    const presentation = resolvePresentationColors(container);
    const fontSize = Number.parseFloat(getComputedStyle(document.documentElement).getPropertyValue("--ad-terminal-font-size"));
    const term = new Terminal({
      fontFamily: presentation.fontFamily,
      fontSize,
      theme: xtermTheme(presentation),
      cursorBlink: true,
      convertEol: false,
    });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(container);
    const safeFit = () => {
      try {
        fit.fit();
      } catch {
        /* container not laid out / measured yet */
      }
    };
    safeFit();

    const proto = window.location.protocol === "https:" ? "wss" : "ws";
    const ws = new WebSocket(`${proto}://${window.location.host}/api/sessions/${agentId}/terminal/ws`);
    ws.binaryType = "arraybuffer";

    const sendResize = () => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ cols: term.cols, rows: term.rows }));
      }
    };

    ws.onopen = () => {
      safeFit();
      sendResize();
    };
    ws.onmessage = (event) => {
      const data = typeof event.data === "string" ? event.data : new Uint8Array(event.data as ArrayBuffer);
      term.write(data);
    };
    ws.onclose = () => term.write("\r\n[closed]\r\n");
    ws.onerror = () => term.write("\r\n[connection error]\r\n");

    // Keystrokes → binary frame; a string send would be misrouted to resize and dropped.
    const dataSub = term.onData((data) => {
      if (ws.readyState === WebSocket.OPEN) ws.send(new TextEncoder().encode(data));
    });
    // Emulator size changes (from fit) → tell the PTY via a {cols,rows} text frame.
    const resizeSub = term.onResize(() => sendResize());

    const onWindowResize = () => safeFit();
    window.addEventListener("resize", onWindowResize);

    return () => {
      window.removeEventListener("resize", onWindowResize);
      dataSub.dispose();
      resizeSub.dispose();
      ws.close();
      term.dispose();
    };
  }, [agentId]);

  return (
    <div className="terminal-panel" data-ui="terminal">
      <div className="terminal-xterm" data-slot="viewport" ref={containerRef} />
    </div>
  );
}
