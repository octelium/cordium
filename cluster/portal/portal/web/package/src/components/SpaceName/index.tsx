import { useAppSelector } from "@/utils/hooks";
import { getShortNameFromRef } from "@/utils/pb";
import * as MetaPB from "@octelium/apis/main/metav1";
import Label from "../Label";

const SpaceName = (props: {
  spaceRef: MetaPB.ObjectReference;
  showType?: boolean;
}) => {
  const { spaceRef, showType } = props;
  const status = useAppSelector((state) => state.settings.status);
  return (
    <>
      <span className="flex flex-row items-center">
        <span>{getShortNameFromRef(spaceRef)}</span>
        {showType && (
          <Label>
            {spaceRef.name.endsWith("cordium") ? "Organization" : "User"}
          </Label>
        )}
      </span>
    </>
  );
};

export default SpaceName;
