import { useState } from "react";
import { configErrorMessage, pickDirectory } from "../../api/config";

interface BrowseDirectoryButtonProps {
  /** Receives the absolute directory the person selected. It is not called when
   * the panel is dismissed or the browse fails, so the field keeps its current
   * value in both cases (FS-04.R42). */
  onPicked: (path: string) => void;
  /** Surfaces a browse failure through the form's existing error line (INV §8). */
  onError: (message: string) => void;
  disabled?: boolean;
  /** Accessible name; one form has more than one of these buttons. */
  label: string;
}

/**
 * BrowseDirectoryButton is the one Browse control behind every first-party
 * project directory field (FS-04.R42) — Settings cwd, the pending additional
 * directory, and onboarding cwd — so the three cannot drift (INV §2). It only
 * reports the chosen path upward: committing it to the form, and saving the
 * project, stay with the form as they were.
 */
export function BrowseDirectoryButton({
  onPicked,
  onError,
  disabled,
  label,
}: BrowseDirectoryButtonProps) {
  const [browsing, setBrowsing] = useState(false);

  const browse = async () => {
    setBrowsing(true);
    try {
      const path = await pickDirectory();
      if (path) onPicked(path);
    } catch (e) {
      onError(configErrorMessage(e));
    } finally {
      setBrowsing(false);
    }
  };

  return (
    <button
      type="button"
      className="btn-link"
      aria-label={label}
      disabled={disabled || browsing}
      onClick={() => void browse()}
    >
      {browsing ? "Browsing…" : "Browse…"}
    </button>
  );
}
