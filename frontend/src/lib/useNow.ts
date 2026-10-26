import { useEffect, useState } from "react";
import { serverNow } from "./clock";

/** The current time on the server's clock, re-rendering every `everyMs`. Drives countdowns. */
export function useNow(everyMs = 1000): number {
  const [now, setNow] = useState(serverNow);
  useEffect(() => {
    const id = setInterval(() => setNow(serverNow()), everyMs);
    return () => clearInterval(id);
  }, [everyMs]);
  return now;
}
