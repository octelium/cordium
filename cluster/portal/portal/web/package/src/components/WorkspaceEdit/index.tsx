import * as React from "react";

import * as WsPB from "../../apis/cordiumv1/cordiumv1";

import { ObjectReference } from "@/apis/metav1/metav1";
import { Tabs } from "@mantine/core";
import {
  cloneResource,
  resourceFromYAML,
  resourceToYAML,
} from "../../utils/pb";
import Editor from "../Editor";
import EditSpec from "./EditSpec";

const WorkspaceEdit = (props: {
  item: WsPB.Workspace | WsPB.Template;
  onUpdate: (item: WsPB.Workspace | WsPB.Template) => void;
  onDone?: (item: WsPB.Workspace | WsPB.Template) => void;
  spaceRef: ObjectReference;
}) => {
  let [req, setReq] = React.useState(props.item);

  const [value, setValue] = React.useState(0);

  const [vYAML, setVYAML] = React.useState<string | undefined>(undefined);

  const updateReq = () => {
    console.log("cloning...", req);
    const clone = cloneResource(req) as WsPB.Workspace | WsPB.Template;
    setReq(clone);
    props.onUpdate(clone);
  };

  React.useEffect(() => {
    return () => {
      if (vYAML) {
        const rsc = resourceFromYAML(vYAML)!;
        console.log("CLONING", rsc);
        req = cloneResource(rsc) as WsPB.Workspace | WsPB.Template;
        console.log(req);
        updateReq();
      }
    };
  }, []);

  return (
    <div>
      <div>
        <Tabs defaultValue="spec" className="font-bold">
          <Tabs.List className="mb-4">
            <Tabs.Tab value="spec">Spec</Tabs.Tab>
            <Tabs.Tab value="yaml">YAML</Tabs.Tab>
          </Tabs.List>

          <Tabs.Panel value="spec">
            <EditSpec
              spaceRef={props.spaceRef}
              item={req}
              onUpdate={(itm) => {
                // setReq(WsPB.Workspace.clone(itm as WsPB.Workspace));
                // console.log("UPDATE", req);
                // req = WsPB.Workspace.clone(itm as WsPB.Workspace);
                req = cloneResource(itm) as WsPB.Workspace | WsPB.Template;
                console.log(req);
                updateReq();
              }}
            />
          </Tabs.Panel>

          <Tabs.Panel value="yaml">
            <Editor
              mode="yaml"
              value={resourceToYAML(cloneResource(req))}
              onChange={(val) => {
                setVYAML(val);
              }}
            />
          </Tabs.Panel>
        </Tabs>
      </div>
    </div>
  );
};

export default WorkspaceEdit;
