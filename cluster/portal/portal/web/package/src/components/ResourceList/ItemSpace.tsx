import { Space } from "@/apis/cordiumv1/cordiumv1";
import { getPathSpace } from "@/utils/octelium";
import { ResourceListItem, ResourceListItemMetadata } from ".";

const ItemSpace = (props: { item: Space }) => {
  const { item } = props;

  return (
    <ResourceListItem key={item.metadata!.uid} path={getPathSpace(item)}>
      <div className="font-semibold w-full">
        <div className="flex items-center">
          <div className="flex flex-col flex-1">
            <ResourceListItemMetadata resource={props.item} />
          </div>
        </div>
      </div>
    </ResourceListItem>
  );
};

export default ItemSpace;
