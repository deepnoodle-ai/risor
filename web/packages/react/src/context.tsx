import { createContext, useContext, useEffect, useMemo } from "react";
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

/** The service's environment for a description, released when it changes or on unmount. */
export function useRisorEnvironment(description: Description | undefined): RisorEnvironment {
  const service = useRisorService();
  const key = descriptionKey(description);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  const env = useMemo(() => service.environment(description), [service, key]);
  useEffect(() => () => env.release(), [env]);
  return env;
}
