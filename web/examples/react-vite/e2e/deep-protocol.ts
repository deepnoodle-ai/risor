// What deep.spec.ts gives deep.html and its worker, and what they give back.

/** A program to run at a depth limit, with the variables of deep-forms.ts. */
export interface DeepCase {
  name: string;
  source: string;
}

/**
 * Two programs: `outer` calls the host function `inner` at its deepest point,
 * which runs `inner` there. The stack of the two together is that of one
 * program as deep as both.
 */
export interface StackedCase {
  name: string;
  outer: string;
  inner: string;
}

export interface DeepRequest<C> {
  /** The variables, as one record type, and their values. */
  vars: string;
  values: Record<string, unknown>;
  depth: number;
  cases: C[];
}

/** For each case by name: the value as Risor's JSON text, or the code of the failure. */
export type DeepResults = Record<string, string>;

export interface RisorDeep {
  /** Run each case through @deepnoodle/risor-service, in its worker. */
  service(req: DeepRequest<DeepCase>): Promise<DeepResults>;
  /** Run each case with @deepnoodle/risor in a module worker of its own. */
  stacked(req: DeepRequest<StackedCase>): Promise<DeepResults>;
}

declare global {
  interface Window {
    /** Set by deep.ts. */
    risorDeep: RisorDeep;
  }
}
