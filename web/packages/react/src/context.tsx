import { createContext, useContext, useEffect, useState } from "react";
import type { ReactNode } from "react";
import { sharedService } from "@risor/codemirror";
import type { Description, RisorEnvironment, RisorService } from "@risor/service";

const ServiceContext = createContext<RisorService | null>(null);

/** Give the components below one service. Without it they share one worker service per page. */
export function RisorProvider({ service, children }: { service: RisorService; children?: ReactNode }) {
  return <ServiceContext.Provider value={service}>{children}</ServiceContext.Provider>;
}

/** The service of the nearest RisorProvider, or the page's shared worker service. */
export function useRisorService(): RisorService {
  return useContext(ServiceContext) ?? sharedService();
}

/**
 * A key for a description, so a description written inline (a new object on
 * each render) does not make a new environment on each render.
 */
export function descriptionKey(d: Description | undefined): string {
  return d === undefined ? "" : JSON.stringify(d);
}

/**
 * The service's environment for a description, opened in an effect and
 * released when the description changes or on unmount. Null until the first
 * effect runs, and on the server. Opening it in an effect, not in render,
 * means a render React discards never opens one.
 */
export function useRisorEnvironment(description: Description | undefined): RisorEnvironment | null {
  const service = useRisorService();
  const key = descriptionKey(description);
  const [env, setEnv] = useState<RisorEnvironment | null>(null);
  useEffect(() => {
    const e = service.environment(description);
    setEnv(e);
    return () => e.release();
    // The key stands for the description.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [service, key]);
  return env;
}
