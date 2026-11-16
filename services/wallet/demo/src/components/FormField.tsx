'use client';

import { useId, useRef, useState, type HTMLAttributes } from 'react';

interface FormFieldProps {
  label: string;
  name: string;
  value: string;
  onValueChange: (value: string) => void;
  errorMessage: string;
  type?: 'text' | 'email';
  inputMode?: HTMLAttributes<HTMLInputElement>['inputMode'];
  autoComplete?: string;
  placeholder?: string;
  pattern?: string;
  multiline?: boolean;
  numeric?: boolean;
  dataInput?: boolean;
  disabled?: boolean;
  accessibleLabel?: string;
  validate?: (value: string) => boolean;
}

export function FormField({
  label,
  name,
  value,
  onValueChange,
  errorMessage,
  type = 'text',
  inputMode,
  autoComplete,
  placeholder,
  pattern,
  multiline,
  numeric,
  dataInput,
  disabled,
  accessibleLabel,
  validate,
}: FormFieldProps) {
  const id = useId();
  const edited = useRef(false);
  const [error, setError] = useState(false);

  function updateValidity(control: HTMLInputElement | HTMLTextAreaElement) {
    control.setCustomValidity(validate && !validate(control.value) ? errorMessage : '');
  }

  const props = {
    id,
    name,
    value,
    required: true,
    disabled,
    inputMode,
    autoComplete,
    placeholder,
    spellCheck: false,
    autoCapitalize: 'none',
    autoCorrect: 'off',
    enterKeyHint: 'done' as const,
    className: `input${dataInput ? ' data-input' : ''}${numeric ? ' number-input' : ''}`,
    'aria-label': accessibleLabel,
    'aria-invalid': error,
    'aria-describedby': error ? `${id}-error` : undefined,
    onChange(event: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) {
      edited.current = true;
      updateValidity(event.currentTarget);
      if (event.currentTarget.validity.valid) setError(false);
      onValueChange(event.currentTarget.value);
    },
    onBlur(event: React.FocusEvent<HTMLInputElement | HTMLTextAreaElement>) {
      if (!edited.current) return;
      updateValidity(event.currentTarget);
      setError(!event.currentTarget.validity.valid);
    },
    onInvalid(event: React.InvalidEvent<HTMLInputElement | HTMLTextAreaElement>) {
      updateValidity(event.currentTarget);
      setError(true);
    },
  };

  return (
    <div className="field">
      <label htmlFor={id}>
        {label} <span aria-hidden="true">*</span>
      </label>
      {multiline ? <textarea {...props} /> : <input {...props} type={type} pattern={pattern} />}
      {error && (
        <p id={`${id}-error`} className="field-error">
          {errorMessage}
        </p>
      )}
    </div>
  );
}
