import { Anchor } from "@mantine/core";
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
import Tag from "../Tag";

const getCheckout = (r: Workspace_Spec_Repository | undefined) =>
  r?.cloneOptions?.branch || r?.cloneOptions?.checkout;

const ProviderIcon = (props: { url: string }) => {
  if (props.url.includes("github.com")) return <IconBrandGithub size={14} />;
  if (props.url.includes("gitlab.com")) return <IconBrandGitlab size={14} />;
  if (props.url.includes("bitbucket.org"))
    return <IconBrandBitbucket size={14} />;
  return <IconGitBranch size={14} />;
};

const RepoLink = (props: { item: Workspace | Template }) => {
  const repository = props.item.spec?.repository;
  const url = repository?.url;
  const checkout = getCheckout(repository);

  if (!url) return null;

  return (
    <span className="inline-flex items-center gap-2 min-w-0">
      <span className="text-slate-400 shrink-0 flex">
        <ProviderIcon url={url} />
      </span>
      <Anchor
        href={url}
        target="_blank"
        rel="noopener noreferrer"
        size="sm"
        fw={500}
        underline="hover"
        className="truncate max-w-[24rem]"
        onClick={(e) => e.stopPropagation()}
      >
        {url.replace(/^https:\/\//, "")}
      </Anchor>
      {checkout && (
        <Tag icon={<IconGitBranch size={11} />} mono>
          {checkout}
        </Tag>
      )}
    </span>
  );
};

export default RepoLink;
