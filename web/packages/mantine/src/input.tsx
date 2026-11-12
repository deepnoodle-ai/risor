import { forwardRef, useId, useImperativeHandle, useRef, useState } from "react";
import type { ReactNode } from "react";
import { Input, useComputedColorScheme } from "@mantine/core";
import type { MantineSize } from "@mantine/core";
import { isEmptyText } from "@risor/codemirror";
import { RisorEditor } from "@risor/react";
import type { Description, Problem, RisorEditorHandle } from "@risor/react";

export interface RisorInputProps {
  value: string;
  /** Called with the new text. Works with @mantine/form's getInputProps. */
  onChange?: (value: string) => void;
  /** The environment: a description (Go's Env.Describe(), env.describe(), or describeSample). */
  environment?: Description;
  /** The type the expression must have, in type text: "bool". */
  expected?: string;
  label?: ReactNode;
  description?: ReactNode;
  /** An error to show. It wins over the checker's message. */
  error?: ReactNode;
  /** Show the checker's first message as the error when `error` is not given. Default true. */
  showCheckErrors?: boolean;
  /** The field must not be empty: `validate()` then gives a problem for empty text. */
  required?: boolean;
  withAsterisk?: boolean;
  placeholder?: string;
  /** Allow more than one line. Default false: Enter calls onSubmit. */
  multiline?: boolean;
  minLines?: number;
  maxLines?: number;
  readOnly?: boolean;
  disabled?: boolean;
  size?: MantineSize;
  /** Called on Enter (one line) or Mod-Enter. */
  onSubmit?: () => void;
  onDiagnostics?: (problems: Problem[]) => void;
  onFocus?: () => void;
  onBlur?: () => void;
  className?: string;
  id?: string;
}

/**
 * A checked Risor expression field in Mantine's input style, with a label,
 * a description and an error. Its ref has `validate()`: await it on submit,
 * since the checker's message lags the text by the pause after typing.
 */
/** The ref of a RisorInput: `validate()`, `format()`, `focus()` and the CodeMirror view. */
export type RisorInputHandle = RisorEditorHandle;

export const RisorInput = forwardRef<RisorInputHandle, RisorInputProps>(function RisorInput(props, ref) {
  const { value, onChange, environment, expected, label, description, error, showCheckErrors = true, required, withAsterisk, placeholder, multiline = false, minLines, maxLines, readOnly, disabled, size = "sm", onSubmit, onDiagnostics, onFocus, onBlur, className } = props;
  const auto = useId();
  const id = props.id ?? `risor-${auto.replace(/:/g, "")}`;
  const scheme = useComputedColorScheme("light");
  const editor = useRef<RisorEditorHandle>(null);
  const [problems, setProblems] = useState<Problem[]>([]);
  const requiredRef = useRef(required);
  requiredRef.current = required;
  useImperativeHandle(ref, () => ({
    validate: async () => {
      let ps = (await editor.current?.validate()) ?? [];
      // An empty text has no problems to the checker; a required field makes it one.
      const text = editor.current?.view?.state.doc.toString() ?? "";
      if (ps.length === 0 && requiredRef.current && isEmptyText(text)) ps = [{ phase: "check", code: "required", message: "Enter an expression." }];
      setProblems(ps);
      return ps;
    },
    format: () => editor.current?.format() ?? Promise.resolve(false),
    focus: () => editor.current?.focus(),
    get view() {
      return editor.current?.view ?? null;
    },
  }), []);

  const shown = error ?? (showCheckErrors && problems.length > 0 ? problems[0].message : undefined);
  const describedBy = [description ? `${id}-description` : "", shown ? `${id}-error` : ""].filter(Boolean).join(" ") || undefined;

  return (
    <Input.Wrapper
      id={id}
      label={label}
      description={description}
      error={shown}
      required={required}
      withAsterisk={withAsterisk}
      size={size}
      className={className}
      labelProps={{ id: `${id}-label`, onClick: () => editor.current?.focus() }}
    >
      <RisorEditor
        ref={editor}
        className="risor-mantine-input"
        style={{ fontSize: `var(--input-fz, var(--mantine-font-size-${size}))` }}
        value={value}
        onChange={onChange}
        environment={environment}
        expected={expected}
        singleLine={!multiline}
        minLines={minLines}
        maxLines={maxLines}
        placeholder={placeholder}
        readOnly={readOnly || disabled}
        colorScheme={scheme}
        onRun={onSubmit}
        onDiagnostics={(ps) => {
          setProblems(ps);
          onDiagnostics?.(ps);
        }}
        onFocus={onFocus}
        onBlur={onBlur}
        aria-labelledby={label ? `${id}-label` : undefined}
        aria-describedby={describedBy}
        aria-invalid={Boolean(shown)}
      />
    </Input.Wrapper>
  );
});
