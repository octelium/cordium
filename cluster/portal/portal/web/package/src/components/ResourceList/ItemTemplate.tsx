import { Template } from "@octelium/apis/main/cordiumv1";
import { ResourceListItem, ResourceListItemMetadata } from ".";
import Repository from "../Repository";

const ItemTemplate = (props: { item: Template }) => {
  const { item } = props;
  return (
    <ResourceListItem
      key={item.metadata!.uid}
      path={`/templates/uid/${item.metadata!.uid}`}
    >
      <div className="font-semibold w-full">
        <div className="flex items-center">
          <div className="flex flex-col flex-1">
            <ResourceListItemMetadata resource={props.item} />
            <div className="w-full">
              <Repository item={props.item} />
            </div>
          </div>
        </div>
      </div>
    </ResourceListItem>
  );
};

export default ItemTemplate;
