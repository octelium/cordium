import InfoItem from "@/components/InfoItem";
import PageWrap from "@/components/PageWrap";
import ResourceYAML from "@/components/ResourceYAML";
import StartWorkspace from "@/components/StartWorkspace";
import TimeAgo from "@/components/TimeAgo";
import { getResourceRef, getShortName } from "@/utils/pb";
import { Text } from "@mantine/core";
import { useContextSpace } from "../utils";

const Page = () => {
  const ctx = useContextSpace();
  const data = ctx.space.data;

  return (
    <PageWrap qry={ctx.space}>
      {data && (
        <div style={{ display: "flex", flexDirection: "column", gap: 24 }}>
          <div
            style={{
              background: "white",
              border: "1px solid #e2e8f0",
              borderRadius: 12,
              overflow: "hidden",
            }}
          >
            <div
              style={{
                padding: "14px 20px",
                borderBottom: "1px solid #e2e8f0",
                background: "#f8fafc",
              }}
            >
              <Text
                size="xs"
                fw={700}
                tt="uppercase"
                style={{ letterSpacing: "0.06em", color: "#94a3b8" }}
              >
                Space details
              </Text>
            </div>

            <div style={{ padding: "4px 20px 8px" }}>
              <InfoItem title="Name">{getShortName(data)}</InfoItem>

              {data.metadata?.displayName && (
                <InfoItem title="Display name">
                  {data.metadata.displayName}
                </InfoItem>
              )}

              <InfoItem title="Created">
                <TimeAgo rfc3339={data.metadata?.createdAt} />
              </InfoItem>

              <InfoItem title="Raw config">
                <ResourceYAML item={data} size="xs" />
              </InfoItem>
            </div>
          </div>

          <StartWorkspace spaceRef={getResourceRef(data)} />
        </div>
      )}
    </PageWrap>
  );
};

export default Page;
