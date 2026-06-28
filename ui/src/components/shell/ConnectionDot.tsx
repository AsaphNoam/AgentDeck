import { useUiStore } from "../../store/uiStore";

export function ConnectionDot() {
  const connection = useUiStore((state) => state.connection);
  return (
    <span className={`connection-dot ${connection}`} aria-label={`SSE ${connection}`} />
  );
}
