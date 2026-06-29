import { useEffect, useRef, useState } from "react";

export function TerminalTab({ agentId }: { agentId: string }) {
  const [lines, setLines] = useState<string[]>([]);
  const [input, setInput] = useState("");
  const wsRef = useRef<WebSocket | null>(null);

  useEffect(() => {
    const proto = window.location.protocol === "https:" ? "wss" : "ws";
    const ws = new WebSocket(`${proto}://${window.location.host}/api/sessions/${agentId}/terminal/ws`);
    ws.binaryType = "arraybuffer";
    wsRef.current = ws;
    ws.onopen = () => setLines((prev) => [...prev, "[connected]"]);
    ws.onmessage = (event) => {
      const text = typeof event.data === "string" ? event.data : new TextDecoder().decode(event.data);
      setLines((prev) => [...prev.slice(-400), text]);
    };
    ws.onclose = () => setLines((prev) => [...prev, "[closed]"]);
    ws.onerror = () => setLines((prev) => [...prev, "[connection error]"]);
    return () => ws.close();
  }, [agentId]);

  const send = () => {
    const ws = wsRef.current;
    if (!ws || ws.readyState !== WebSocket.OPEN || input === "") return;
    // Keystrokes must reach the PTY as a *binary* frame: the bridge routes text
    // frames to resize ({cols,rows}) and only forwards binary frames to the PTY
    // master, so a string send would be silently dropped (review fix).
    ws.send(new TextEncoder().encode(`${input}\n`));
    setInput("");
  };

  return (
    <div className="terminal-panel">
      <pre>{lines.join("")}</pre>
      <div className="terminal-input">
        <input
          value={input}
          onChange={(event) => setInput(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === "Enter") send();
          }}
        />
        <button type="button" onClick={send}>Send</button>
      </div>
    </div>
  );
}
