import { useEffect, useRef, useState } from "react";

function capitalize(s: string): string {
  if (!s) return s;
  return s.charAt(0).toUpperCase() + s.slice(1);
}

function suggest(role: string, project: string): string {
  if (!role || !project) return "";
  return `${capitalize(role)}-${project}`;
}

export function useSuggestedName(
  role: string,
  project: string,
): [string, (v: string) => void] {
  const [name, setName] = useState(() => suggest(role, project));
  const dirtyRef = useRef(false);

  useEffect(() => {
    if (!dirtyRef.current) {
      setName(suggest(role, project));
    }
  }, [role, project]);

  const handleChange = (v: string) => {
    dirtyRef.current = true;
    setName(v);
  };

  return [name, handleChange];
}
