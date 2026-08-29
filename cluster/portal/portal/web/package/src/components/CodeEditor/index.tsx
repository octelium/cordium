import { langs } from "@uiw/codemirror-extensions-langs";
import CodeMirror from "@uiw/react-codemirror";
import * as React from "react";

const CodeEditor = (props: {
  value: string;
  mode?: "yaml" | "dockerfile" | "shell";
  onChange?: (val: string) => void;
  readOnly?: boolean;
  minHeight?: string;
  maxHeight?: string;
  autoFocus?: boolean;
}) => {
  const extensions = React.useMemo(() => {
    switch (props.mode) {
      case "yaml":
        return [langs.yaml()];
      case "shell":
      case "dockerfile":
        return [langs.bash()];
      default:
        return [];
    }
  }, [props.mode]);

  return (
    <div className="console-surface rounded-lg border border-slate-800 overflow-hidden">
      <CodeMirror
        value={props.value}
        readOnly={props.readOnly}
        autoFocus={props.autoFocus}
        theme="dark"
        minHeight={props.minHeight ?? "200px"}
        maxHeight={props.maxHeight ?? "520px"}
        extensions={extensions}
        onChange={props.onChange}
      />
    </div>
  );
};

export default CodeEditor;
