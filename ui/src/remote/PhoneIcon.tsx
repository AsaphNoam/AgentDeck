import type { ReactNode } from "react";

// Companion icons are local vectors from the supplied mobile study.
export function PhoneIcon({ name, size = 20 }: { name: string; size?: number }) {
  const paths: Record<string, ReactNode> = {
    home: <><path d="m3 10 9-7 9 7v10H3Z" /><path d="M9 20v-7h6v7" /></>,
    phone: <><rect x="6" y="2" width="12" height="20" rx="3" /><path d="M10 18h4" /></>,
    agent: <><rect x="4" y="6" width="16" height="14" rx="4" /><path d="M12 2v4M8 11v2m8-2v2M9 16h6" /></>,
    pipeline: <><rect x="3" y="3" width="6" height="6" rx="1" /><rect x="15" y="15" width="6" height="6" rx="1" /><path d="M9 6h8v9M6 9v9h9" /></>,
    arrow: <path d="m9 6 6 6-6 6" />, back: <path d="m15 6-6 6 6 6" />,
    bell: <><path d="M5 17h14l-2-3V9a5 5 0 0 0-10 0v5Z" /><path d="M10 21h4" /></>,
    folder: <path d="M3 7V4h7l2 3h9v13H3Z" />,
    check: <path d="m5 12 4 4L19 6" />, plus: <path d="M12 5v14M5 12h14" />,
    menu: <path d="M4 6h16M4 12h16M4 18h16" />, up: <path d="M12 20V4m-7 7 7-7 7 7" />,
    shield: <><path d="m12 3 8 3v6c0 5-8 9-8 9s-8-4-8-9V6Z" /><path d="m8 11 3 3 5-5" /></>,
    wifi: <><path d="M3 8a15 15 0 0 1 18 0M6 12a10 10 0 0 1 12 0M9 16a5 5 0 0 1 6 0" /><circle cx="12" cy="20" r=".5" /></>,
    file: <><path d="M5 3h9l5 5v13H5Z" /><path d="M14 3v6h5M8 13h8M8 17h5" /></>,
  };
  return <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.65" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">{paths[name] ?? paths.agent}</svg>;
}
