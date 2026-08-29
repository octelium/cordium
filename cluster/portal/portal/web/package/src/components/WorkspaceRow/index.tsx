import { getPathWorkspace } from "@/utils/octelium";
import { getShortNameFromRef } from "@/utils/pb";
import { Workspace } from "@octelium/apis/main/cordiumv1";
import {
  IconBolt,
  IconStack2,
  IconTemplate,
} from "@tabler/icons-react";
import { CardTitle, ClickableCard } from "../ResourceCards";
import RepoLink from "../RepoLink";
import StateBadge from "../StateBadge";
import Tag from "../Tag";
import TimeAgo from "../TimeAgo";

const WorkspaceRow = (props: {
  item: Workspace;
  showSpace?: boolean;
  showTemplate?: boolean;
}) => {
  const { item } = props;

  return (
    <ClickableCard to={getPathWorkspace(item)}>
      <div className="flex flex-col gap-3 md:flex-row md:items-center">
        <div className="min-w-0 flex-1">
          <CardTitle
            name={item.metadata!.name}
            displayName={item.metadata?.displayName}
            meta={
              <>
                Created <TimeAgo rfc3339={item.metadata?.createdAt} />
                {item.status?.lastActivityAt && (
                  <>
                    {" · Active "}
                    <TimeAgo rfc3339={item.status.lastActivityAt} />
                  </>
                )}
              </>
            }
          />

          {item.spec?.repository?.url && (
            <div className="mt-1.5">
              <RepoLink item={item} />
            </div>
          )}

          <div className="mt-2 flex flex-wrap gap-1.5">
            {props.showSpace && item.status?.spaceRef && (
              <Tag icon={<IconStack2 size={11} />} label="Space">
                {getShortNameFromRef(item.status.spaceRef)}
              </Tag>
            )}
            {props.showTemplate && item.status?.templateRef && (
              <Tag icon={<IconTemplate size={11} />} label="Template">
                {getShortNameFromRef(item.status.templateRef)}
              </Tag>
            )}
            {item.spec?.isEphemeral && (
              <Tag tone="warning" icon={<IconBolt size={11} />}>
                Ephemeral
              </Tag>
            )}
          </div>
        </div>

        <div className="shrink-0">
          <StateBadge state={item.status!.state} />
        </div>
      </div>
    </ClickableCard>
  );
};

export default WorkspaceRow;
