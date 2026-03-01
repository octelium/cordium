import InfoItem from "@/components/InfoItem";
import PageWrap from "@/components/PageWrap";
import ResourceYAML from "@/components/ResourceYAML";
import StartWorkspace from "@/components/StartWorkspace";
import TimeAgo from "@/components/TimeAgo";
import { getResourceRef, getShortName } from "@/utils/pb";
import { useContextSpace } from "../utils";

const Page = () => {
  const ctx = useContextSpace();

  const data = ctx.space.data;

  return (
    <PageWrap qry={ctx.space}>
      {data && (
        <div>
          {<InfoItem title="Name">{getShortName(data)}</InfoItem>}

          {data.metadata?.displayName && (
            <InfoItem title="Display Name">
              {data.metadata.displayName}
            </InfoItem>
          )}
          {/**
           <InfoItem title="Type">
            <Label>
              {data.status?.type === Space_Status_Type.ORGANIZATION
                ? "Organization"
                : "User"}
            </Label>
          </InfoItem>
           **/}
          <InfoItem title="Created">
            <TimeAgo rfc3339={data.metadata?.createdAt} />
          </InfoItem>
          <InfoItem title="Detailed Info">
            <ResourceYAML item={data} size="xs" />
          </InfoItem>

          <StartWorkspace spaceRef={getResourceRef(data)} />
        </div>
      )}
    </PageWrap>
  );
};

export default Page;
