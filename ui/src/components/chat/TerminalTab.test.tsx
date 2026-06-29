import React from "react";
import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { render, act, cleanup } from "@testing-library/react";

// xterm.js touches canvas/DOM measurement that jsdom doesn't implement, so we
// mock it with a light fake that lets the test fire onData/onResize and inspect
// the WebSocket frames the component sends.
const xtermState = vi.hoisted(() => ({
  instances: [] as Array<{
    cols: number;
    rows: number;
    fireData: (d: string) => void;
    fireResize: () => void;
  }>,
}));

vi.mock("@xterm/xterm/css/xterm.css", () => ({}));
vi.mock("@xterm/addon-fit", () => ({
  FitAddon: class {
    fit() {}
  },
}));
vi.mock("@xterm/xterm", () => ({
  Terminal: class {
    cols = 80;
    rows = 24;
    private dataCb: ((d: string) => void) | null = null;
    private resizeCb: (() => void) | null = null;
    constructor() {
      xtermState.instances.push({
        cols: this.cols,
        rows: this.rows,
        fireData: (d) => this.dataCb?.(d),
        fireResize: () => this.resizeCb?.(),
      });
    }
    loadAddon() {}
    open() {}
    write() {}
    dispose() {}
    onData(cb: (d: string) => void) {
      this.dataCb = cb;
      return { dispose() {} };
    }
    onResize(cb: () => void) {
      this.resizeCb = cb;
      return { dispose() {} };
    }
  },
}));

import { TerminalTab } from "./TerminalTab";

class FakeWebSocket {
  static OPEN = 1;
  static instances: FakeWebSocket[] = [];
  readyState = FakeWebSocket.OPEN;
  binaryType = "arraybuffer";
  sent: unknown[] = [];
  onopen: (() => void) | null = null;
  onmessage: ((e: { data: unknown }) => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;
  constructor(public url: string) {
    FakeWebSocket.instances.push(this);
  }
  send(data: unknown) {
    this.sent.push(data);
  }
  close() {}
}

beforeEach(() => {
  xtermState.instances = [];
  FakeWebSocket.instances = [];
  vi.stubGlobal("WebSocket", FakeWebSocket as unknown as typeof WebSocket);
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("TerminalTab", () => {
  it("sends keystrokes to the PTY as binary frames (not text frames)", () => {
    render(<TerminalTab agentId="a_1" />);
    const ws = FakeWebSocket.instances[0];
    const term = xtermState.instances[0];

    act(() => term.fireData("ls\r"));

    expect(ws.sent).toHaveLength(1);
    const frame = ws.sent[0];
    // A binary frame is what the bridge forwards to the PTY master; a string
    // would be misrouted to the resize handler and silently dropped.
    expect(typeof frame).not.toBe("string");
    expect(ArrayBuffer.isView(frame as ArrayBufferView)).toBe(true);
    expect(new TextDecoder().decode(frame as Uint8Array)).toBe("ls\r");
  });

  it("sends a {cols,rows} text frame on open and on resize", () => {
    render(<TerminalTab agentId="a_1" />);
    const ws = FakeWebSocket.instances[0];
    const term = xtermState.instances[0];

    act(() => ws.onopen?.());
    expect(ws.sent.at(-1)).toBe(JSON.stringify({ cols: 80, rows: 24 }));

    ws.sent.length = 0;
    act(() => term.fireResize());
    expect(ws.sent).toHaveLength(1);
    expect(ws.sent[0]).toBe(JSON.stringify({ cols: 80, rows: 24 }));
    // resize is a text frame (string), never binary
    expect(typeof ws.sent[0]).toBe("string");
  });
});
