import { forwardRef, useEffect, useImperativeHandle, useRef, useState } from "react";
import type { CSSProperties } from "react";
import { closeBrackets, closeBracketsKeymap, completionKeymap } from "@codemirror/autocomplete";
import { defaultKeymap, history, historyKeymap, indentWithTab } from "@codemirror/commands";
import { Compartment, EditorState } from "@codemirror/state";
import type { Extension } from "@codemirror/state";
import { drawSelection, EditorView, keymap, placeholder as placeholderExt } from "@codemirror/view";
import { formatRisor, risor, validateRisor } from "@risor/codemirror";
import type { Description, Problem } from "@risor/service";
import { descriptionKey, useRisorService } from "./context.tsx";

export interface RisorEditorProps {
  value: string;
  onChange?: (value: string) => void;
  /** The environment: a description (Go's Env.Describe(), env.describe(), or describeSample). */
  environment?: Description;
  /** The type the text must have, in type text: "bool". */
  expected?: string;
  /** Keep the text on one line; Enter calls onRun. */
  singleLine?: boolean;
  onDiagnostics?: (problems: Problem[]) => void;
  /** Called on Mod-Enter, and on Enter in single-line mode. */
  onRun?: () => void;
  /** The height, in lines, the editor has when its text is shorter. Default 1 for single-line, else 4. */
  minLines?: number;
  /** The height, in lines, past which the editor scrolls. Default: none. */
  maxLines?: number;
  placeholder?: string;
  readOnly?: boolean;
  autoFocus?: boolean;
  /** "light", "dark", or follow the page's `prefers-color-scheme` ("auto", the default). */
  colorScheme?: "light" | "dark" | "auto";
  /** More CodeMirror extensions, made once. */
  extensions?: Extension;
  className?: string;
  style?: CSSProperties;
  id?: string;
  "aria-label"?: string;
  "aria-labelledby"?: string;
  "aria-describedby"?: string;
  "aria-invalid"?: boolean;
  onFocus?: () => void;
  onBlur?: () => void;
}

export interface RisorEditorHandle {
  /** Check the text now, skipping the pause, and show its problems. Await it on submit. */
  validate(): Promise<Problem[]>;
  /** Format the text. False when it does not parse. */
  format(): Promise<boolean>;
  focus(): void;
  /** The CodeMirror view, once mounted. */
  readonly view: EditorView | null;
}

const LINE = 1.5; // em

function sizeTheme(minLines: number, maxLines: number | undefined): Extension {
  return EditorView.theme({
    ".cm-content, .cm-gutter": { minHeight: `${minLines * LINE}em` },
    ".cm-scroller": { lineHeight: `${LINE}`, fontFamily: "var(--risor-font-mono, ui-monospace, SFMono-Regular, Menlo, monospace)", ...(maxLines ? { maxHeight: `${maxLines * LINE}em`, overflow: "auto" } : {}) },
  });
}

function prefersDark(): boolean {
  return typeof matchMedia === "function" && matchMedia("(prefers-color-scheme: dark)").matches;
}

/**
 * A Risor editor: colours, errors as you type, completion, hover, format
 * (Shift-Alt-F) and run (Mod-Enter). Controlled: give `value` and
 * `onChange`. On the server and before it mounts it renders the text in a
 * <pre> of the same size.
 */
export const RisorEditor = forwardRef<RisorEditorHandle, RisorEditorProps>(function RisorEditor(props, ref) {
  const { value, environment, expected, singleLine = false, minLines, maxLines, placeholder, readOnly = false, autoFocus, colorScheme = "auto", className, style } = props;
  const service = useRisorService();
  const host = useRef<HTMLDivElement>(null);
  const view = useRef<EditorView | null>(null);
  const [mounted, setMounted] = useState(false);
  // Callbacks change on each render; the extension reads them through this ref.
  const latest = useRef(props);
  latest.current = props;
  const compartments = useRef({ risor: new Compartment(), size: new Compartment(), editable: new Compartment(), scheme: new Compartment(), placeholder: new Compartment() });
  const lines = minLines ?? (singleLine ? 1 : 4);

  const risorExtension = () =>
    risor({
      service,
      environment,
      expected,
      singleLine,
      onDiagnostics: (p) => latest.current.onDiagnostics?.(p),
      onRun: () => latest.current.onRun?.(),
    });

  const dark = colorScheme === "dark" || (colorScheme === "auto" && prefersDark());

  useEffect(() => {
    const c = compartments.current;
    const v = new EditorView({
      parent: host.current!,
      state: EditorState.create({
        doc: value,
        extensions: [
          history(),
          drawSelection(),
          closeBrackets(),
          keymap.of([...closeBracketsKeymap, ...completionKeymap, ...defaultKeymap, ...historyKeymap, indentWithTab]),
          singleLine ? [] : EditorView.lineWrapping,
          c.risor.of(risorExtension()),
          c.size.of(sizeTheme(lines, maxLines)),
          c.editable.of([EditorView.editable.of(!readOnly), EditorState.readOnly.of(readOnly)]),
          c.scheme.of(EditorView.darkTheme.of(dark)),
          c.placeholder.of(placeholder ? placeholderExt(placeholder) : []),
          EditorView.updateListener.of((u) => {
            if (u.docChanged) latest.current.onChange?.(u.state.doc.toString());
            if (u.focusChanged) (u.view.hasFocus ? latest.current.onFocus : latest.current.onBlur)?.();
          }),
          EditorView.contentAttributes.of({
            ...(props.id ? { id: props.id } : {}),
            ...(props["aria-label"] ? { "aria-label": props["aria-label"] } : {}),
            ...(props["aria-labelledby"] ? { "aria-labelledby": props["aria-labelledby"] } : {}),
            ...(props["aria-describedby"] ? { "aria-describedby": props["aria-describedby"] } : {}),
          }),
          props.extensions ?? [],
        ],
      }),
    });
    view.current = v;
    setMounted(true);
    if (autoFocus) v.focus();
    return () => {
      v.destroy();
      view.current = null;
    };
    // The view is made once; the effects below carry later prop changes into it.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // The value, when the parent changes it.
  useEffect(() => {
    const v = view.current;
    if (v && v.state.doc.toString() !== value) v.dispatch({ changes: { from: 0, to: v.state.doc.length, insert: value } });
  }, [value]);

  const key = `${descriptionKey(environment)}\u0000${expected ?? ""}\u0000${singleLine}`;
  const firstKey = useRef(key);
  useEffect(() => {
    const v = view.current;
    if (!v || key === firstKey.current) return;
    firstKey.current = key;
    v.dispatch({ effects: compartments.current.risor.reconfigure(risorExtension()) });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key, service]);

  useEffect(() => {
    view.current?.dispatch({ effects: compartments.current.size.reconfigure(sizeTheme(lines, maxLines)) });
  }, [lines, maxLines]);
  useEffect(() => {
    view.current?.dispatch({ effects: compartments.current.editable.reconfigure([EditorView.editable.of(!readOnly), EditorState.readOnly.of(readOnly)]) });
  }, [readOnly]);
  useEffect(() => {
    view.current?.dispatch({ effects: compartments.current.placeholder.reconfigure(placeholder ? placeholderExt(placeholder) : []) });
  }, [placeholder]);
  useEffect(() => {
    const apply = () => view.current?.dispatch({ effects: compartments.current.scheme.reconfigure(EditorView.darkTheme.of(colorScheme === "dark" || (colorScheme === "auto" && prefersDark()))) });
    apply();
    if (colorScheme !== "auto" || typeof matchMedia !== "function") return;
    const mq = matchMedia("(prefers-color-scheme: dark)");
    mq.addEventListener("change", apply);
    return () => mq.removeEventListener("change", apply);
  }, [colorScheme]);

  useImperativeHandle(ref, () => ({
    validate: () => (view.current ? validateRisor(view.current) : Promise.resolve([])),
    format: () => (view.current ? formatRisor(view.current) : Promise.resolve(false)),
    focus: () => view.current?.focus(),
    get view() {
      return view.current;
    },
  }), []);

  const classes = ["risor-editor", singleLine ? "risor-editor-single" : "", props["aria-invalid"] ? "risor-editor-invalid" : "", className ?? ""].filter(Boolean).join(" ");
  return (
    <div className={classes} style={style} data-risor-editor="">
      {!mounted && (
        <pre className="risor-editor-fallback" style={{ minHeight: `${lines * LINE}em`, lineHeight: LINE, margin: 0 }}>
          {value || placeholder}
        </pre>
      )}
      <div ref={host} />
    </div>
  );
});
