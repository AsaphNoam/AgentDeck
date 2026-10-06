import type { SVGProps } from "react";

// Inline action icons (FS-02.R67, TS-08.R89). They inherit `currentColor` and are decorative:
// the button that holds one carries the accessible name and tooltip.
function Icon({ children, ...props }: SVGProps<SVGSVGElement>) {
  return (
    <svg className="ad-icon" viewBox="0 0 16 16" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" focusable="false" {...props}>
      {children}
    </svg>
  );
}

export function SendIcon() {
  return <Icon><path d="M8 13V3M3.5 7.5 8 3l4.5 4.5" /></Icon>;
}

export function StopIcon() {
  return <Icon><rect x="4" y="4" width="8" height="8" rx="1" fill="currentColor" stroke="none" /></Icon>;
}

export function CollapseIcon() {
  return <Icon><path d="M4 10 8 6l4 4" /></Icon>;
}

export function CollapseAllIcon() {
  return <Icon><path d="M4 8.5 8 4.5l4 4M4 12.5l4-4 4 4" /></Icon>;
}
