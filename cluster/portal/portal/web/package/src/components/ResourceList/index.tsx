import { Resource, getShortName } from "@/utils/pb";
import { Button } from "@mantine/core";
import { useNavigate } from "react-router-dom";
import { twMerge } from "tailwind-merge";
import TimeAgo from "../TimeAgo";

export const ResourceListWrapper = (props: { children?: React.ReactNode }) => {
  return <div className="flex flex-col w-full">{props.children}</div>;
};

export const ResourceListItem = (props: {
  children?: React.ReactNode;
  path?: string;
}) => {
  const hasPath = props.path !== undefined && props.path.length > 0;
  const navigate = useNavigate();
  return (
    <div
      className={twMerge(
        "w-full",
        hasPath ? "cursor-pointer" : undefined,
        "transition-all duration-300",
        "bg-white",
        "hover:bg-transparent",
        "py-4 px-2",
        "font-semibold",
        "rounded-xl",
        "shadow-sm shadow-slate-200",
        "border-2 border-slate-300",
        "mb-4",
      )}
      onClick={() => {
        if (hasPath) {
          navigate(props.path!);
        }
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
    <div className="flex flex-col flex-1">
      <div className="flex items-center font-bold">
        {!props.noName && (
          <span className="text-gray-800 mr-2">{getShortName(resource)}</span>
        )}
        {md.displayName && (
          <span className="text-gray-600">{md.displayName}</span>
        )}
      </div>

      {/*
      {md.uid && (
        <div className="flex items-center text-xs font-bold">
          <div className="text-gray-800 mr-2">UID</div>
          <div className="text-gray-400">{md.uid}</div>
        </div>
      )} 
      */}
      {md.createdAt && (
        <div className="flex items-center text-xs text-gray-500 font-bold">
          <span className="mr-1">Created</span>
          <span>
            <TimeAgo rfc3339={md.createdAt} />{" "}
          </span>
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
    <div className="flex justify-end mb-6">
      <Button
        size="sm"
        onClick={() => {
          navigate(props.path ? props.path : "create");
        }}
      >
        {props.title}
      </Button>
    </div>
  );
};
