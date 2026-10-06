import { forwardRef, useCallback, useEffect, useImperativeHandle, useLayoutEffect, useRef, type TextareaHTMLAttributes } from "react";

export interface AutoGrowTextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  /** CSS length that caps growth; beyond it the field scrolls (composers pass `40vh`). */
  maxHeight?: string;
}

// The one textarea for ui/src (FS-02.R66, TS-08.R88): no resize grip, and its height follows its
// content. `rows`/CSS min-height stay the floor because the measure starts from `height: auto`.
export const AutoGrowTextarea = forwardRef<HTMLTextAreaElement, AutoGrowTextareaProps>(function AutoGrowTextarea(
  { maxHeight, style, onInput, ...props },
  ref,
) {
  const inner = useRef<HTMLTextAreaElement>(null);
  useImperativeHandle(ref, () => inner.current as HTMLTextAreaElement, []);

  const fit = useCallback(() => {
    const el = inner.current;
    if (!el) return;
    el.style.height = "auto";
    // A hidden field measures 0; leave it at its natural height until it can be measured.
    if (!el.scrollHeight) return;
    el.style.height = `${el.scrollHeight + el.offsetHeight - el.clientHeight}px`;
  }, []);

  // Every render, so controlled values, form resets and default values all refit.
  useLayoutEffect(fit);
  useEffect(() => {
    window.addEventListener("resize", fit);
    return () => window.removeEventListener("resize", fit);
  }, [fit]);

  return (
    <textarea
      ref={inner}
      style={{ ...style, maxHeight, overflowY: maxHeight ? "auto" : "hidden" }}
      onInput={(event) => {
        fit();
        onInput?.(event);
      }}
      {...props}
    />
  );
});
