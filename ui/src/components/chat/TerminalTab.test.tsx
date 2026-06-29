import React from "react";
import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { render, screen, fireEvent, act, cleanup } from "@testing-library/react";
import { TerminalTab } from "./TerminalTab";

// Minimal fake WebSocket that records what was sent and lets the test drive open().
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
  FakeWebSocket.instances = [];
  vi.stubGlobal("WebSocket", FakeWebSocket as unknown as typeof WebSocket);
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("TerminalTab", () => {
  it("sends typed input to the PTY as a binary frame (not a text frame)", () => {
    render(<TerminalTab agentId="a_1" />);
    const ws = FakeWebSocket.instances[0];
    act(() => ws.onopen?.());

    fireEvent.change(screen.getByRole("textbox"), { target: { value: "ls -la" } });
    fireEvent.click(screen.getByRole("button", { name: /Send/i }));

    expect(ws.sent).toHaveLength(1);
    const frame = ws.sent[0];
    // A binary frame is what the bridge forwards to the PTY master; a string
    // would be misrouted to the resize handler and silently dropped.
    expect(typeof frame).not.toBe("string");
    expect(ArrayBuffer.isView(frame as ArrayBufferView)).toBe(true);
    expect(new TextDecoder().decode(frame as Uint8Array)).toBe("ls -la\n");
  });

  it("also sends a binary frame when Enter is pressed", () => {
    render(<TerminalTab agentId="a_1" />);
    const ws = FakeWebSocket.instances[0];
    act(() => ws.onopen?.());

    const input = screen.getByRole("textbox");
    fireEvent.change(input, { target: { value: "echo hi" } });
    fireEvent.keyDown(input, { key: "Enter" });

    expect(ws.sent).toHaveLength(1);
    expect(typeof ws.sent[0]).not.toBe("string");
    expect(ArrayBuffer.isView(ws.sent[0] as ArrayBufferView)).toBe(true);
    expect(new TextDecoder().decode(ws.sent[0] as Uint8Array)).toBe("echo hi\n");
  });
});
