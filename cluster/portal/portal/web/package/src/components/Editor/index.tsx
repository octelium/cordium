import { langs } from "@uiw/codemirror-extensions-langs";
import CodeMirror from "@uiw/react-codemirror";
import React from "react";

const Editor = (props: {
  value: string;
  mode: "yaml" | "dockerfile" | "shell" | undefined;
  onChange?: (val: string) => void;
  onFinalChange?: (val: string) => void;
  readOnly?: boolean;
}) => {
  let extensions;
  switch (props.mode) {
    case "yaml": {
      extensions = [langs.yaml()];
      break;
    }
    case "dockerfile": {
      break;
    }
    case "shell": {
      extensions = [langs.bash()];
      break;
    }
  }

  let [cur, setCur] = React.useState<string | undefined>(undefined);

  React.useEffect(() => {
    return () => {
      if (cur && props.onFinalChange) {
        props.onFinalChange(cur);
      }
    };
  }, []);

  return (
    <div className="font-bold rounded-xl border-2 overflow-hidden w-full shadow-2xl text-xs">
      <CodeMirror
        value={props.value}
        autoFocus={true}
        readOnly={props.readOnly}
        className="w-full"
        theme={"dark"}
        maxHeight="600px"
        minHeight="300px"
        extensions={extensions}
        onChange={(val) => {
          setCur(val);
          if (props.onChange) {
            props.onChange(val);
          }
        }}
      />
    </div>
  );
};

export default Editor;
