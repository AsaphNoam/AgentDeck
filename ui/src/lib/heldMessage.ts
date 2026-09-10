import { withdrawPrompt } from "../api/client";
import { useHeldStore } from "../store/heldStore";

// Two surfaces offer withdrawing the queued follow-up — the composer (FS-03.R48)
// and the transcript's pending tail (TS-08.R56) — so the server call and the
// client release live in one helper rather than drifting apart in two components
// (INV §2). The client state is released only after the server confirms, so a
// failed withdraw leaves the message visibly pending instead of silently
// disappearing from a chat that is still going to send it (INV §8).
export async function withdrawHeldMessage(agentId: string): Promise<void> {
  await withdrawPrompt(agentId);
  useHeldStore.getState().release(agentId);
}
