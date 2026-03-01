import * as React from "react";

import { useAppDispatch } from "@/utils/hooks";

import { getClientWorkspace } from "@/utils/client";

import { onError } from "@/utils";

import * as WsPB from "@/apis/cordiumv1/cordiumv1";

import WorkspaceStatus from "@/components/WorkspaceStatus";
import { BsFillStopFill } from "react-icons/bs";
import { FaPlay } from "react-icons/fa";

import { useNavigate } from "react-router-dom";

import { GetOptions } from "@/apis/metav1/metav1";
import CopyText from "@/components/CopyText";
import InfoItem from "@/components/InfoItem";
import Label from "@/components/Label";
import Repository from "@/components/Repository";
import TimeAgo from "@/components/TimeAgo";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { twMerge } from "tailwind-merge";

import { BiLinkExternal } from "react-icons/bi";

import BoxItem from "@/components/BoxItem";
import LinkWrap from "@/components/LinkWrap";
import PageWrap from "@/components/PageWrap";
import ResourceYAML from "@/components/ResourceYAML";
import SpaceName from "@/components/SpaceName";
import { clearTerminalGroup } from "@/features/terminalgroup/slice";
import {
  getPathSpace,
  getPathTemplate,
  invalidateResource,
} from "@/utils/octelium";
import { canStopWorkspace, getResourceRef, getShortName } from "@/utils/pb";

import { Button, Modal } from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import axios from "axios";
import { useContextWorkspace } from "../utils";

interface AuthBegin {
  loginURL: string;
}

const LoginGitProvider = (props: { item: WsPB.Workspace }) => {
  const { item } = props;
  const client = getClientWorkspace();
  if (
    !item.status?.templateRef ||
    item.status.spaceType !== WsPB.Space_Status_Type.ORGANIZATION
  ) {
    return <></>;
  }

  if (item.status!.state !== WsPB.Workspace_Status_State.STOPPED) {
    return <></>;
  }

  const qryTemplate = useQuery({
    queryKey: ["workspace/getTemplate", item.status!.templateRef!.uid],

    queryFn: () => {
      const { response } = getClientWorkspace().getTemplate(
        GetOptions.create({ uid: item.status!.templateRef!.uid }),
      );
      return response;
    },
  });

  const mutation = useMutation({
    mutationFn: async () => {
      const resp = await axios.post<AuthBegin>(
        `/auth/v1/begin/${item.metadata!.uid}`,
      );
      return resp.data;
    },
    onSuccess: (data) => {
      window.location.href = data.loginURL;
    },
    onError: onError,
  });

  if (!qryTemplate.isSuccess) {
    return <></>;
  }

  if (!qryTemplate.data.status?.gitProviderRef) {
    return <></>;
  }

  return (
    <button
      className={twMerge(
        "flex items-center justify-center cursor-pointer",
        `transition-all duration-300`,
        "w-full bg-gray-800 text-white font-bold rounded-lg py-4 px-4 text-xl",
        "hover:bg-black",
        "shadow-2xl",
      )}
      onClick={() => {
        mutation.mutate();
      }}
    >
      Login to Git Provider
    </button>
  );
};

const StartStopButton = (props: { item: WsPB.Workspace }) => {
  const dispatch = useAppDispatch();
  const navigate = useNavigate();

  const client = getClientWorkspace();
  const queryClient = useQueryClient();

  const { item } = props;
  const canStop = canStopWorkspace(item);

  let [openStop, setOpenStop] = React.useState(false);
  let [openStart, setOpenStart] = React.useState(false);

  const [opened, { open, close }] = useDisclosure(false);

  let [startWorkspaceRequest, setStartWorkspaceRequest] =
    React.useState<WsPB.StartWorkspaceRequest>(
      WsPB.StartWorkspaceRequest.create({
        uid: item.metadata!.uid,
      }),
    );

  let [stopWorkspaceRequest, setStopWorkspaceRequest] =
    React.useState<WsPB.StopWorkspaceRequest>(
      WsPB.StopWorkspaceRequest.create({
        uid: item.metadata!.uid,
      }),
    );

  const mutationStop = useMutation({
    mutationFn: async () => {
      const { response } = await client.stopWorkspace(stopWorkspaceRequest);
      return response;
    },
    onSuccess: (response) => {
      close();
      dispatch(clearTerminalGroup({}));
      invalidateResource(item);
    },
    onError: () => {
      close();
    },
  });

  const mutationStart = useMutation({
    mutationFn: async () => {
      const { response } = await client.startWorkspace(startWorkspaceRequest);
      return response;
    },
    onSuccess: (response) => {
      invalidateResource(item);
    },
  });

  return (
    <>
      {canStop && (
        <button
          className={twMerge(
            "flex items-center justify-center cursor-pointer",
            `transition-all duration-300`,
            "w-full text-black font-bold rounded-lg py-4 px-4 text-xl",
            "border-2 border-black",
            "hover:bg-white",
            "shadow-xl",
          )}
          onClick={() => {
            // mutationStop.mutate();
            open();
          }}
        >
          <span className="mr-1">Stop</span>
          <BsFillStopFill />
        </button>
      )}

      {item.status!.state === WsPB.Workspace_Status_State.STOPPED && (
        <div className="w-full flex flex-col items-center justify-center">
          <button
            className={twMerge(
              "flex items-center justify-center cursor-pointer",
              `transition-all duration-500`,
              "w-full bg-gray-800 text-white font-bold rounded-lg py-4 px-4 text-xl",
              "hover:bg-black",
              "shadow-2xl",
            )}
            onClick={() => {
              mutationStart.mutate();
            }}
          >
            <span className="mr-1">Start</span>
            <FaPlay />
          </button>

          {/*
          <button
            className={twMerge(
              "mt-4",
              "flex items-center justify-center",
              `transition-all duration-300`,
              " text-gray-800 font-bold rounded-lg p-2 text-xs",
              "hover:text-black"
            )}
            onClick={() => {
              setOpenStart(true);
            }}
          >
            <span className="mr-1">Start with Options</span>
          </button>
          */}
        </div>
      )}

      {/*
      <Dialog
        open={openStart}
        onClose={() => {
          setOpenStart(false);
        }}
        aria-labelledby="alert-dialog-title"
        aria-describedby="alert-dialog-description"
      >
        <DialogTitle id="alert-dialog-title">
          <div className="ml-4">
            <div className="font-bold mb-2 text-lg">Run Options</div>

            {item.status && item.status.runs.length > 0 && (
              <div className="w-full p4">
                <Autocomplete
                  value={startWorkspaceRequest.fromRunID}
                  options={item.status.runs
                    .filter((x) => !x.isEphemeral && !x.failure && x.stoppedAt)
                    .map((x) => x.id)}
                  sx={{ width: 300 }}
                  onChange={(v, b) => {
                    startWorkspaceRequest.fromRunID = b ?? "";
                    setStartWorkspaceRequest(
                      WsPB.StartWorkspaceRequest.clone(startWorkspaceRequest)
                    );
                  }}
                  renderInput={(params) => <TextField {...params} />}
                />
              </div>
            )}
          </div>
        </DialogTitle>

        <DialogActions>
          <Button
            size="small"
            mode="outline"
            onClick={() => {
              setOpenStart(false);
            }}
          >
            Cancel
          </Button>
          <Button
            size="small"
            onClick={() => {
              mutationStart.mutate();
              setOpenStart(false);
            }}
            autoFocus
          >
            Start Workspace
          </Button>
        </DialogActions>
      </Dialog>
      */}

      <Modal
        opened={opened}
        onClose={() => {
          close();
        }}
        centered
      >
        <div className="font-bold text-xl mb-4">
          {`Are you sure that you want to Stop this Workspace?`}
        </div>

        <div className="w-full my-4">
          <InfoItem title="Name">{props.item.metadata!.name}</InfoItem>
          <InfoItem title="UID">{props.item.metadata!.uid}</InfoItem>
          <InfoItem title="Detailed Info">
            <ResourceYAML item={item} size="xs" />
          </InfoItem>
        </div>

        <div className="mt-4 flex justify-end items-center">
          <Button
            variant="outline"
            onClick={() => {
              close();
            }}
          >
            Cancel
          </Button>
          <Button
            className={twMerge("ml-4  transition-all duration-500")}
            loading={mutationStop.isPending}
            // color="red"
            onClick={() => {
              mutationStop.mutate();
            }}
            autoFocus
          >
            Yes, Stop Workspace
          </Button>
        </div>
      </Modal>
    </>
  );
};

const InfoBar = (props: { item: WsPB.Workspace }) => {
  const item = props.item;
  const isActive = item.status?.state !== WsPB.Workspace_Status_State.STOPPED;

  const qryTemplate = useQuery({
    queryKey: ["workspace/getTemplate", item.status!.templateRef!.uid],

    queryFn: () => {
      const { response } = getClientWorkspace().getTemplate(
        GetOptions.create({ uid: item.status!.templateRef!.uid }),
      );
      return response;
    },
  });

  const qrySpace = useQuery({
    queryKey: ["workspace/getSpace", item.status!.spaceRef!.uid],

    queryFn: () => {
      const { response } = getClientWorkspace().getSpace(
        GetOptions.create({ uid: item.status!.spaceRef!.uid }),
      );
      return response;
    },
  });

  return (
    <div className="w-full mb-4">
      <div>
        <div className="flex flex-col md:flex-row">
          <div className="flex md:basis-2/3 md:mr-1">
            <div>
              <InfoItem title="Name">
                <div className="flex items-center">
                  <CopyText value={item.metadata!.name} />
                </div>
              </InfoItem>
              {item.metadata?.displayName && (
                <InfoItem title="Display Name">
                  {item.metadata?.displayName}
                </InfoItem>
              )}

              {qrySpace.isSuccess && (
                <InfoItem title="Space">
                  <div className="flex items-center">
                    <LinkWrap to={getPathSpace(qrySpace.data!)}>
                      <SpaceName spaceRef={getResourceRef(qrySpace.data!)} />
                    </LinkWrap>
                  </div>
                </InfoItem>
              )}

              {qryTemplate.isSuccess && (
                <InfoItem title="Template">
                  <div className="flex items-center">
                    <LinkWrap to={getPathTemplate(qryTemplate.data)}>
                      {getShortName(qryTemplate.data)}
                    </LinkWrap>
                  </div>
                </InfoItem>
              )}

              <InfoItem title="Detailed Info">
                <ResourceYAML item={item} size="xs" />
              </InfoItem>

              {isActive && (
                <InfoItem title="URL">
                  <a
                    className={twMerge(
                      "flex items-center justify-center text-sm font-bold cursor-pointer",
                      "text-cyan-800 hover:text-gray-800 rounded-full transition-all duration-300 shadow-2xl",
                    )}
                    href={
                      isActive ? `https://${item.status!.hostname}` : undefined
                    }
                    target="_blank"
                  >
                    {`https://${item.status!.hostname}`}
                    <span className="ml-1">
                      <BiLinkExternal />
                    </span>
                  </a>
                </InfoItem>
              )}

              <InfoItem title="Created">
                <TimeAgo rfc3339={item.metadata?.createdAt} />
              </InfoItem>

              <InfoItem title="State">
                <WorkspaceStatus status={item.status!.state} />
              </InfoItem>
              {item.status?.isEphemeral && (
                <InfoItem title="Ephemeral">
                  <span className={"text-rose-700"}>YES</span>
                </InfoItem>
              )}

              {item.status?.lastInitializedAt && (
                <InfoItem title="Last Initialized">
                  <TimeAgo rfc3339={item.status?.lastInitializedAt} />
                </InfoItem>
              )}

              {item.status?.lastStoppedAt && (
                <InfoItem title="Last Stopped">
                  <TimeAgo rfc3339={item.status?.lastStoppedAt} />
                </InfoItem>
              )}

              {item.status?.state === WsPB.Workspace_Status_State.RUNNING &&
                item.status?.lastActivityAt && (
                  <InfoItem title="Last Activity">
                    <TimeAgo rfc3339={item.status?.lastActivityAt} />
                  </InfoItem>
                )}

              {item.status?.limit &&
                (item.status.limit.cpu || item.status.limit.memory) && (
                  <InfoItem title="Resource Limit">
                    {item.status.limit.cpu && (
                      <Label>
                        {item.status.limit.cpu.millicores / 1000} CPU
                      </Label>
                    )}
                    {item.status.limit.memory &&
                      (item.status.limit.memory.megabytes >= 1000 ? (
                        <Label>
                          {item.status.limit.memory.megabytes / 1000}GB RAM
                        </Label>
                      ) : (
                        <Label>
                          {item.status.limit.memory.megabytes}MB RAM
                        </Label>
                      ))}

                    {item.status.limit.storage &&
                      (item.status.limit.storage.megabytes >= 1000 ? (
                        <Label>
                          {item.status.limit.storage.megabytes / 1000}GB Storage
                        </Label>
                      ) : (
                        <Label>
                          {item.status.limit.storage.megabytes}MB Storage
                        </Label>
                      ))}
                  </InfoItem>
                )}

              <div>
                <Repository item={item} />
              </div>
            </div>
          </div>
          <div className="flex flex-col items-center justify-center md:basis-1/3">
            <StartStopButton item={item} />
            <div className="my-2"></div>
            <LoginGitProvider item={item} />
          </div>
        </div>
      </div>
    </div>
  );
};

const AppItem = (props: {
  app: WsPB.Workspace_Spec_Application;
  item: WsPB.Workspace;
}) => {
  const { item, app } = props;

  return (
    <a
      className="flex items-center justify-center text-sm font-bold mx-1 my-1 py-1 px-2 cursor-pointer border-[1px] border-gray-400 hover:bg-gray-200 rounded-full transition-all duration-300 shadow-2xl"
      href={
        item.status?.hostname
          ? app.isDefault
            ? `https://${item.status!.hostname}`
            : `https://${app.name}_${item.status!.hostname}`
          : undefined
      }
      target="_blank"
    >
      <div className="mr-1">
        {app.displayName ? app.displayName : app.name}{" "}
      </div>

      {app.port > 0 && <Label>{app.port}</Label>}
      {app.isDefault && <Label>Default</Label>}
    </a>
  );
};

const AppsBar = (props: { item: WsPB.Workspace }) => {
  const { item } = props;

  const appsWorkspace = item.spec?.applications;

  if (!appsWorkspace?.length) {
    return <></>;
  }

  return (
    <BoxItem title="Applications">
      <div>
        <div className="my-1">
          {appsWorkspace && appsWorkspace.length > 0 && (
            <div className="w-full">
              <div className="mb-2">Workspace Apps</div>
              <div className="flex">
                {appsWorkspace?.map((app) => (
                  <AppItem app={app} item={item} />
                ))}
              </div>
            </div>
          )}
        </div>
      </div>
    </BoxItem>
  );
};

const Page = () => {
  const ctx = useContextWorkspace();
  return (
    <PageWrap qry={ctx.workspace}>
      {ctx.workspace.data && <InfoBar item={ctx.workspace.data} />}
    </PageWrap>
  );
};

export default Page;
