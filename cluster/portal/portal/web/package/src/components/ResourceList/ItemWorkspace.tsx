import { Workspace } from "@/apis/cordiumv1/cordiumv1";
import { useAppSelector } from "@/utils/hooks";
import { ResourceListItem, ResourceListItemMetadata } from ".";
import Label from "../Label";
import Repository from "../Repository";
import WorkspaceStatus from "../WorkspaceStatus";

import { getPathWorkspace } from "@/utils/octelium";
import { getShortNameFromRef } from "@/utils/pb";
import { FaAngleRight } from "react-icons/fa6";
import SpaceName from "../SpaceName";

const ItemWorkspace = (props: {
  item: Workspace;
  showSpace?: boolean;
  showEnvironment?: boolean;
  showTemplate?: boolean;
}) => {
  const { item } = props;
  const status = useAppSelector((state) => state.settings.status);

  return (
    <ResourceListItem key={item.metadata!.uid} path={getPathWorkspace(item)}>
      <div className="font-semibold w-full">
        <div className="flex flex-col md:flex-row md:items-center">
          <div className="flex flex-col flex-1">
            <ResourceListItemMetadata resource={props.item} />
            <div className="w-full">
              <Repository item={props.item} />
            </div>
            <div className="w-full flex mt-1">
              {props.showSpace && (
                <Label>
                  <span className="flex flex-row items-center justify-center">
                    Space
                    <FaAngleRight />{" "}
                    <SpaceName spaceRef={item.status!.spaceRef!} />
                  </span>
                </Label>
              )}
              {item.status?.isEphemeral && <Label>Ephemeral</Label>}

              {props.showTemplate && item.status?.templateRef && (
                <Label>
                  <span className="flex flex-row items-center justify-center">
                    Template
                    <FaAngleRight />{" "}
                    {getShortNameFromRef(item.status?.templateRef)}
                  </span>
                </Label>
              )}
            </div>
          </div>
          <div className="flex-none font-bold text-gray-600">
            <WorkspaceStatus status={props.item.status!.state} />
          </div>
        </div>
      </div>
    </ResourceListItem>
  );
};

export default ItemWorkspace;
