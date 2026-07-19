import EmptyList from "@/components/EmptyList";
import { ResourceListWrapper } from "@/components/ResourceList";
import { TemplateList } from "@octelium/apis/main/cordiumv1";
import { useNavigate } from "react-router-dom";
import Label from "../Label";
import ItemTemplate from "../ResourceList/ItemTemplate";

const TemplateListC = (props: {
  itemList: TemplateList;
  environmentUID?: string;
  seeAllPath?: string;
}) => {
  const { itemList } = props;
  const navigate = useNavigate();
  return (
    <div className="w-full flex flex-col">
      <div className="font-bold text-lg mb-8 flex items-center justify-center">
        <span>Templates</span>
        <Label>Total: {itemList.listResponseMeta?.totalCount}</Label>
      </div>
      <div className="w-full">
        <ResourceListWrapper>
          {itemList.items.length === 0 && (
            <EmptyList title="No Templates Found"></EmptyList>
          )}
          {props.itemList.items.map((item) => (
            <ItemTemplate key={item.metadata?.uid} item={item} />
          ))}
        </ResourceListWrapper>
      </div>
    </div>
  );
};

export default TemplateListC;
