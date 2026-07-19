import { Anchor, Badge, Group } from "@mantine/core";
import {
  Template,
  Workspace,
  Workspace_Spec_Repository,
} from "@octelium/apis/main/cordiumv1";
import {
  IconBrandBitbucket,
  IconBrandGithub,
  IconBrandGitlab,
  IconGitBranch,
} from "@tabler/icons-react";

const getCheckout = (r: Workspace_Spec_Repository | undefined) =>
  r?.cloneOptions?.branch ?? r?.cloneOptions?.checkout;

const ProviderIcon = (props: { url: string }) => {
  if (props.url.includes("github.com")) return <IconBrandGithub size={14} />;
  if (props.url.includes("gitlab.com")) return <IconBrandGitlab size={14} />;
  if (props.url.includes("bitbucket.org"))
    return <IconBrandBitbucket size={14} />;
  return <IconGitBranch size={14} />;
};

export const hasRepository = (item: Workspace | Template): boolean => {
  if (item.kind === "Workspace")
    return !!(item as Workspace).spec?.repository?.url;
  if (item.kind === "Template")
    return !!(item as Template).spec?.repository?.url;
  return false;
};

const Repository = (props: { item: Workspace | Template }) => {
  const { item } = props;

  const spec =
    item.kind === "Template"
      ? (item as Template).spec
      : item.kind === "Workspace"
        ? (item as Workspace).spec
        : undefined;

  const repository = spec?.repository?.url;
  const checkout = getCheckout(spec?.repository);

  if (!repository) return null;

  return (
    <Group gap="xs" wrap="nowrap">
      <span style={{ color: "var(--mantine-color-dimmed)", display: "flex" }}>
        <ProviderIcon url={repository} />
      </span>
      <Anchor
        href={repository}
        target="_blank"
        rel="noopener noreferrer"
        size="sm"
        fw={500}
        underline="hover"
        style={{
          overflow: "hidden",
          textOverflow: "ellipsis",
          whiteSpace: "nowrap",
          maxWidth: 360,
        }}
      >
        {repository}
      </Anchor>
      {checkout && (
        <Badge
          size="sm"
          variant="outline"
          color="gray"
          leftSection={<IconGitBranch size={10} />}
        >
          {checkout}
        </Badge>
      )}
    </Group>
  );
};

export default Repository;
