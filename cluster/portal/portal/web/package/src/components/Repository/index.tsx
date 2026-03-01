import {
  Template,
  Workspace,
  Workspace_Spec_Repository,
} from "@/apis/cordiumv1/cordiumv1";
import { BsGithub } from "react-icons/bs";
import { IoGitBranch, IoLogoBitbucket, IoLogoGitlab } from "react-icons/io5";
import Label from "../Label";

const getCheckout = (
  r: Workspace_Spec_Repository | undefined,
): string | undefined => {
  return r?.cloneOptions?.branch ?? r?.cloneOptions?.checkout;
};

const GetIcon = (props: { repository?: string }) => {
  const { repository } = props;

  if (repository?.includes("github.com")) {
    return <BsGithub />;
  } else if (repository?.includes("gitlab.com")) {
    return <IoLogoGitlab />;
  } else if (repository?.includes("bitbucket.org")) {
    return <IoLogoBitbucket />;
  }

  return <></>;
};

const Repository = (props: {
  item: Workspace | Template;
  withItem?: boolean;
}) => {
  const { item } = props;
  let repository: string | undefined;
  let checkout: string | undefined;

  if (item.kind === `Template`) {
    const itm = item as Template;
    repository = itm.spec?.repository?.url;
    checkout = getCheckout(itm.spec?.repository);
  } else if (item.kind === `Workspace`) {
    const itm = item as Workspace;
    repository = itm.spec?.repository?.url;
    checkout = getCheckout(itm.spec?.repository);
  }

  if (repository === undefined) {
    return <></>;
  }

  return (
    <span className="text-gray-600 font-semibold text-sm flex items-center">
      <IoGitBranch />
      <a
        className="ml-1 cursor-pointer hover:text-gray-800 duration-300 transition-all"
        target="_blank"
        rel="noopener noreferrer"
        onClick={(e) => {
          e.preventDefault();
          e.stopPropagation();
          window.open(repository, "_blank", "noopener,noreferrer");
        }}
      >
        {repository}
      </a>
      {checkout && (
        <Label outlined>
          <span className="flex items-center justify-center">{checkout}</span>
        </Label>
      )}
    </span>
  );
};

export default Repository;
