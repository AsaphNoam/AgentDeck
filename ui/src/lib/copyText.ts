// The agent header's Copy thread identity (FS-03.R62) and the transcript's Copy
// selection (FS-03.R63) share one clipboard path so the availability check and
// the "Copy failed" wording cannot drift apart (INV §2). Failures surface as a
// toast rather than silently doing nothing (INV §8).
export function copyText(text: string, pushError: (title: string, body: string) => void): void {
  if (!navigator.clipboard) {
    pushError("Copy failed", "Clipboard access is not available in this browser.");
    return;
  }
  void navigator.clipboard.writeText(text).catch((error) => {
    pushError("Copy failed", error instanceof Error ? error.message : String(error));
  });
}
