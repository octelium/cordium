import { WorkspaceList } from "@/apis/cordiumv1/cordiumv1";
import EmptyList from "@/components/EmptyList";
import { ResourceListWrapper } from "@/components/ResourceList";
import { useNavigate } from "react-router-dom";
import Label from "../Label";
import ItemWorkspace from "../ResourceList/ItemWorkspace";

const WorkspaceListC = (props: {
  itemList: WorkspaceList;
  showEnvironment?: boolean;
  showTemplate?: boolean;
  seeAllPath?: string;
}) => {
  const { itemList } = props;
  const navigate = useNavigate();
  return (
    <div className="w-full flex flex-col">
      <div className="font-bold text-lg mb-8 flex items-center justify-center">
        <span>Your Workspaces</span>
        <Label>Total: {itemList.listResponseMeta?.totalCount}</Label>
      </div>

      <div className="w-full">
        <ResourceListWrapper>
          {itemList.items.length === 0 && (
            <EmptyList title="No Workspaces found"></EmptyList>
          )}
          {props.itemList.items.map((item) => (
            <ItemWorkspace
              key={item.metadata?.uid}
              item={item}
              showEnvironment={props.showEnvironment}
              showTemplate={props.showTemplate}
            />
          ))}
        </ResourceListWrapper>
      </div>
    </div>
  );
};

export default WorkspaceListC;
