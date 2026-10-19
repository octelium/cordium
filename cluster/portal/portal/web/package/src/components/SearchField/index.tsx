import { TextInput } from "@mantine/core";
import { Search, X } from "lucide-react";
import * as React from "react";

const SearchField = (props: {
  value: string;
  onChange: (value: string) => void;
  label: string;
  placeholder?: string;
  className?: string;
}) => {
  const { value, onChange } = props;
  const [text, setText] = React.useState(value);
  const [lastValue, setLastValue] = React.useState(value);
  const inputRef = React.useRef<HTMLInputElement>(null);
  const onChangeRef = React.useRef(onChange);

  React.useEffect(() => {
    onChangeRef.current = onChange;
  }, [onChange]);

  if (value !== lastValue) {
    setLastValue(value);
    setText(value);
  }

  React.useEffect(() => {
    if (text === value) return;
    const timeout = window.setTimeout(() => onChangeRef.current(text), 220);
    return () => window.clearTimeout(timeout);
  }, [text, value]);

  const clear = () => {
    setText("");
    onChangeRef.current("");
    inputRef.current?.focus();
  };

  return (
    <TextInput
      ref={inputRef}
      type="search"
      className={props.className}
      aria-label={props.label}
      placeholder={props.placeholder}
      value={text}
      spellCheck={false}
      autoComplete="off"
      enterKeyHint="search"
      onChange={(event) => setText(event.currentTarget.value)}
      onKeyDown={(event) => {
        if (event.key === "Escape" && text) {
          event.preventDefault();
          clear();
        }
      }}
      leftSection={<Search size={15} aria-hidden />}
      leftSectionPointerEvents="none"
      rightSection={
        text && (
          <button
            type="button"
            aria-label="Clear search"
            onClick={clear}
            className="flex h-6 w-6 items-center justify-center rounded text-ink-muted outline-none hover:bg-surface-strong hover:text-ink focus-visible:ring-2 focus-visible:ring-focus"
          >
            <X size={14} aria-hidden />
          </button>
        )
      }
    />
  );
};

export default SearchField;
