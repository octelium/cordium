import {
  Resource,
  cloneResource,
  resourceFromYAML,
  resourceToYAML,
} from "@/utils/pb";
import * as React from "react";
import Editor from "../Editor";

import { Button } from "@mantine/core";
import { useNavigate } from "react-router-dom";

const EditorResource = (props: {
  item: Resource;
  doneContent: React.ReactNode;
  onDone: (item: Resource) => void;
}) => {
  const navigate = useNavigate();
  const [vYAML, setVYAML] = React.useState<string | undefined>(undefined);

  return (
    <div>
      <Editor
        mode="yaml"
        value={resourceToYAML(cloneResource(props.item))}
        onChange={(val) => {
          setVYAML(val);
        }}
      />

      <div>
        <div>
          <Button
            variant="outline"
            onClick={() => {
              navigate(-1);
            }}
          >
            Cancel
          </Button>

          <Button
            onClick={() => {
              if (vYAML) {
                const rsc = resourceFromYAML(vYAML)!;
                props.onDone(rsc);
              }
            }}
          >
            {props.doneContent}
          </Button>
        </div>
      </div>
    </div>
  );
};

export default EditorResource;
