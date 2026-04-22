import { Resource, getShortName } from "@/utils/pb";
import { Button } from "@mantine/core";
import { Plus } from "lucide-react";
import { useNavigate } from "react-router-dom";
import { twMerge } from "tailwind-merge";
import TimeAgo from "../TimeAgo";

export const ResourceListWrapper = (props: { children?: React.ReactNode }) => (
  <div className="flex flex-col w-full gap-2">{props.children}</div>
);

export const ResourceListItem = (props: {
  children?: React.ReactNode;
  path?: string;
}) => {
  const hasPath = !!props.path?.length;
  const navigate = useNavigate();

  return (
    <div
      className={twMerge(
        "w-full bg-white",
        "border border-slate-200 rounded-xl",
        "shadow-[0_1px_4px_rgba(15,23,42,0.06)]",
        "px-5 py-4",
        "transition-[border-color,box-shadow] duration-150",
        "hover:border-slate-300 hover:shadow-[0_2px_12px_rgba(15,23,42,0.09)]",
        hasPath && "cursor-pointer",
      )}
      onClick={() => {
        if (hasPath) navigate(props.path!);
      }}
    >
      {props.children}
    </div>
  );
};

export const ResourceListItemMetadata = (props: {
  resource: Resource;
  noName?: boolean;
}) => {
  const { resource } = props;
  const md = resource.metadata!;

  return (
    <div className="flex flex-col gap-0.5">
      <div className="flex items-center gap-2">
        {!props.noName && (
          <span className="text-sm font-bold text-slate-800">
            {getShortName(resource)}
          </span>
        )}
        {md.displayName && (
          <span className="text-sm font-semibold text-slate-500">
            {md.displayName}
          </span>
        )}
      </div>
      {md.createdAt && (
        <div className="text-[0.72rem] font-semibold text-slate-400">
          Created <TimeAgo rfc3339={md.createdAt} />
        </div>
      )}
    </div>
  );
};

export const ResourceListCreateItem = (props: {
  title: string;
  path?: string;
}) => {
  const navigate = useNavigate();

  return (
    <div className="flex justify-end mb-4">
      <Button
        size="sm"
        variant="filled"
        color="dark"
        leftSection={<Plus size={13} strokeWidth={2.5} />}
        onClick={() => navigate(props.path ?? "create")}
        styles={{ root: { fontFamily: "Ubuntu, sans-serif", fontWeight: 700 } }}
      >
        {props.title}
      </Button>
    </div>
  );
};
