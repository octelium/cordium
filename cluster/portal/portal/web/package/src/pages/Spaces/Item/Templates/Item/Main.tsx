import Facts, { Fact } from "@/components/Facts";
import LaunchWorkspace from "@/components/LaunchWorkspace";
import Panel, { PanelBody, PanelHeader } from "@/components/Panel";
import RepoLink from "@/components/RepoLink";
import Tag from "@/components/Tag";
import TimeAgo from "@/components/TimeAgo";
import { formatMegabytes, formatMillicores } from "@/utils";
import { getPathSpace } from "@/utils/octelium";
import { getResourceRef, getShortName, getShortNameFromStr } from "@/utils/pb";
import { Anchor, Stack } from "@mantine/core";
import { Link } from "react-router-dom";
import { useContextSpace } from "@/pages/Spaces/utils";

const Page = () => {
  const ctx = useContextSpace();
  const data = ctx.template.data;
  const space = ctx.space.data;

  if (!data || !space) return null;

  const spec = data.spec;
  const image =
    spec?.image?.type.oneofKind === "registry"
      ? spec.image.type.registry.url
      : spec?.image?.type.oneofKind
        ? `Built from ${spec.image.type.oneofKind}`
        : undefined;

  return (
    <Stack gap="lg">
      <div className="grid gap-4 lg:grid-cols-[1fr_20rem]">
        <Panel>
          <PanelHeader title="Blueprint" />
          <PanelBody className="px-5 py-1">
            <Facts>
              <Fact label="Name">
                <span className="font-mono">{getShortName(data)}</span>
              </Fact>
              <Fact label="Space">
                <Anchor
                  component={Link}
                  to={getPathSpace(space)}
                  size="sm"
                  fw={600}
                >
                  {getShortName(space)}
                </Anchor>
              </Fact>
              {image && <Fact label="Image">{image}</Fact>}
              {spec?.repository?.url && (
                <Fact label="Repository">
                  <RepoLink item={data} />
                </Fact>
              )}
              {spec?.gitProvider && (
                <Fact label="Git provider">
                  {getShortNameFromStr(spec.gitProvider)}
                </Fact>
              )}
              {spec?.limit && (
                <Fact label="Resource limits">
                  {[
                    spec.limit.cpu?.millicores
                      ? formatMillicores(spec.limit.cpu.millicores)
                      : null,
                    spec.limit.memory?.megabytes
                      ? formatMegabytes(spec.limit.memory.megabytes)
                      : null,
                    spec.limit.storage?.megabytes
                      ? `${formatMegabytes(spec.limit.storage.megabytes)} disk`
                      : null,
                  ]
                    .filter(Boolean)
                    .join(" · ") || "—"}
                </Fact>
              )}
              {(spec?.vars.length ?? 0) > 0 && (
                <Fact label="Variables">
                  <span className="flex flex-wrap gap-1.5">
                    {spec!.vars.map((v) => (
                      <Tag key={v.name} mono>
                        {v.name}
                        {v.value ? `=${v.value}` : ""}
                      </Tag>
                    ))}
                  </span>
                </Fact>
              )}
              <Fact label="Created">
                <TimeAgo rfc3339={data.metadata?.createdAt} />
              </Fact>
            </Facts>
          </PanelBody>
        </Panel>

        <Panel>
          <PanelHeader title="Runtime summary" />
          <PanelBody className="px-5 py-1">
            <Facts>
              <Fact label="Env vars">
                {spec?.runtime?.envVars.length ?? 0}
              </Fact>
              <Fact label="Tasks">{spec?.runtime?.tasks.length ?? 0}</Fact>
              <Fact label="Features">
                {spec?.runtime?.devcontainers?.features.length ?? 0}
              </Fact>
              <Fact label="Extra repos">
                {spec?.additionalRepositories.length ?? 0}
              </Fact>
            </Facts>
          </PanelBody>
        </Panel>
      </div>

      <LaunchWorkspace
        spaceRef={getResourceRef(space)}
        templateRef={getResourceRef(data)}
      />
    </Stack>
  );
};

export default Page;
