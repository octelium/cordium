import InfoItem from "@/components/InfoItem";
import LinkWrap from "@/components/LinkWrap";
import PageWrap from "@/components/PageWrap";
import Repository from "@/components/Repository";
import ResourceYAML from "@/components/ResourceYAML";
import SpaceName from "@/components/SpaceName";
import StartWorkspace from "@/components/StartWorkspace";
import TimeAgo from "@/components/TimeAgo";
import { useContextSpace } from "@/pages/Spaces/utils";
import { getPathSpace } from "@/utils/octelium";
import { getResourceRef, getShortName } from "@/utils/pb";

export default () => {
  const ctx = useContextSpace();

  const data = ctx.template.data;
  return (
    <PageWrap qry={ctx.template}>
      {data && (
        <div>
          <InfoItem title="Name">{getShortName(data)}</InfoItem>
          {data.metadata?.displayName && (
            <InfoItem title="Display Name">
              {data.metadata?.displayName}
            </InfoItem>
          )}
          <InfoItem title="Created">
            <TimeAgo rfc3339={data.metadata?.createdAt} />
          </InfoItem>

          {ctx.space.data && (
            <InfoItem title="Space">
              <LinkWrap to={getPathSpace(ctx.space.data)}>
                <SpaceName spaceRef={getResourceRef(ctx.space.data)} />
              </LinkWrap>
            </InfoItem>
          )}

          <InfoItem title="Detailed Info">
            <ResourceYAML item={data} size="xs" />
          </InfoItem>

          {data.spec?.repository?.url && (
            <InfoItem title="Git Repo">
              <Repository item={data} />
            </InfoItem>
          )}

          <div>
            <StartWorkspace
              templateRef={getResourceRef(data)}
              disableChooseTemplate
            />
          </div>
        </div>
      )}
    </PageWrap>
  );
};
