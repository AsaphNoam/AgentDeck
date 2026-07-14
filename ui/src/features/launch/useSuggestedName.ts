import { useEffect, useRef, useState } from "react";

function capitalize(s: string): string {
  if (!s) return s;
  return s.charAt(0).toUpperCase() + s.slice(1);
}

function suggest(role: string): string {
  // The default agent name is just the role (FS-01.R1 only requires that the
  // modal auto-suggests some name; the user can always override it).
  if (!role) return "";
  return capitalize(role);
}

export function useSuggestedName(
  role: string,
): [string, (v: string) => void] {
  const [name, setName] = useState(() => suggest(role));
  const dirtyRef = useRef(false);

  useEffect(() => {
    if (!dirtyRef.current) {
      setName(suggest(role));
    }
  }, [role]);

  const handleChange = (v: string) => {
    dirtyRef.current = true;
    setName(v);
  };

  return [name, handleChange];
}
