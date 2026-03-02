import Terminal from "@/components/Terminal";
import { useAppDispatch, useAppSelector } from "@/utils/hooks";

import {
  addTerminal,
  initTerminalGroup,
  removeTerminal,
  setActiveTerminal,
} from "@/features/terminalgroup/slice";

import * as WsPB from "@/apis/cordiumv1/cordiumv1";

import TerminalT from "@/utils/types/terminal";

import EmptyList from "@/components/EmptyList";

import { twJoin, twMerge } from "tailwind-merge";

import { getClientWorkspaceSvc } from "@/utils/client";
import { useQuery } from "@tanstack/react-query";

import InfoItem from "@/components/InfoItem";
import PageWrap from "@/components/PageWrap";
import { truncateUtf8 } from "@/utils";
import { getResourceRef } from "@/utils/pb";
import TerminalI from "@/utils/types/terminal";
import { Button } from "@mantine/core";
import { BiLinkExternal } from "react-icons/bi";
import { IoCloseSharp } from "react-icons/io5";
import { MdAdd } from "react-icons/md";
import { useContextWorkspace } from "../utils";
import { canUseTerminals } from "./utils";

const TabGroup = (props: { workspace: WsPB.Workspace }) => {
  const { workspace } = props;
  const dispatch = useAppDispatch();

  const tg = useAppSelector((state) => state.terminalGroup);

  const wsC = getClientWorkspaceSvc(workspace.status?.regionRef);

  return (
    <>
      <div className="font-bold flex mb-8 w-full">
        <div className="flex-1 font-bold grid grid-cols-4 gap-1 w-full items-center overflow-x-hide mr-4">
          {tg.terminals.map((t, idx) => (
            <div
              key={t.id}
              className={twMerge(
                "w-full shrink flex items-center justify-center border-b-4 cursor-pointer text-sm",
                "py-2 px-2",
                "mx-2",
                // "max-w-[100px]",

                "transition-all duration-300",
                "text-gray-600 hover:text-gray-800 border-b-transparent",
                tg.activeTerminal == t.id
                  ? `border-b-black text-black`
                  : undefined,
              )}
              onClick={() => {
                dispatch(
                  setActiveTerminal({
                    id: t.id,
                  }),
                );
              }}
            >
              <div className="w-full flex-1 flex justify-between items-center">
                <div className="flex-1 overflow-hidden">
                  {truncateUtf8(t.title, 26, { suffix: "..." })}
                </div>
                <button
                  className="cursor-pointer"
                  onClick={async () => {
                    const { response } = await wsC.removeTerminal(
                      WsPB.RemoveTerminalRequest.create({
                        id: t.id,
                      }),
                    );

                    /*
                    if (tg.activeTerminal === t.id) {
                      dispatch(
                        setLastActiveTerminal({
                          id: t.id,
                        })
                      );
                    } else {
                      dispatch(
                        deleteLastActive({
                          id: t.id,
                        })
                      );
                    }
                    */

                    dispatch(removeTerminal({ id: t.id }));
                  }}
                >
                  <IoCloseSharp className="text-xs" />
                </button>
              </div>
            </div>
          ))}
        </div>
        <div>
          <button
            className={twMerge(
              "flex items-center justify-center border-b-4 cursor-pointer text-sm",
              "py-2 px-4",
              "mx-2",
              "border-transparent",
              "transition-all duration-300",
              "text-white bg-gray-800 hover:bg-black rounded-lg shadow-xl",
            )}
            onClick={async () => {
              const { response } = await wsC.createTerminal(
                WsPB.CreateTerminalRequest.create({
                  workspaceRef: getResourceRef(workspace),
                }),
              );
              dispatch(
                addTerminal({
                  id: response.id,
                } as TerminalT),
              );
              dispatch(setActiveTerminal({ id: response.id }));
            }}
          >
            <MdAdd />
          </button>
        </div>
      </div>
    </>
  );
};

const TerminalGroupC = (props: { workspace: WsPB.Workspace }) => {
  const item = props.workspace;

  const wsC = getClientWorkspaceSvc(item.status?.regionRef);
  const tg = useAppSelector((state) => state.terminalGroup);
  const settings = useAppSelector((state) => state.settings);
  const dispatch = useAppDispatch();

  const canUseTerminal = canUseTerminals(item);

  if (!canUseTerminal) {
    return <EmptyList title="Workspace needs to be ready to use Terminals" />;
  }

  const qryListTerm = useQuery({
    queryKey: ["workspace/ws/listTerminal", item.metadata!.uid],
    gcTime: 0,
    queryFn: async () => {
      const { response } = await wsC.listTerminal(
        WsPB.ListTerminalRequest.create({
          workspaceRef: getResourceRef(item),
        }),
      );

      console.log("Got listTerminal", response);

      dispatch(
        initTerminalGroup({
          termList: response.items.map((x) => {
            return {
              id: x.id,
              title: "Terminal",
            } as TerminalI;
          }),
        }),
      );
      return response;
    },
    enabled: canUseTerminal,
  });

  if (!qryListTerm.isSuccess) {
    return <></>;
  }

  /*
  let tg = useAppSelector((state) =>
    state.terminalGroups?.items.find((x) => x.id === item.metadata?.uid)
  );
  */

  /*
  if (tg === undefined) {
    return <></>;
  }
  */

  /*
  if (
    item.status!.state === WsPB.Workspace_Status_State.STOPPED ||
    item.status!.state === WsPB.Workspace_Status_State.STOPPING
  ) {
    return <></>;
  }
  */

  if (tg.terminals.length < 1) {
    return (
      <div className="w-full">
        <EmptyList title="No Terminals Found">
          <div>
            {
              <Button
                onClick={async () => {
                  const { response } = await wsC.createTerminal(
                    WsPB.CreateTerminalRequest.create({
                      workspaceRef: getResourceRef(item),
                    }),
                  );
                  dispatch(
                    addTerminal({
                      id: response.id,
                    } as TerminalT),
                  );
                  dispatch(setActiveTerminal({ id: response.id }));
                }}
              >
                Create Terminal
              </Button>
            }
          </div>
        </EmptyList>
      </div>
    );
  }

  return (
    <div className="my-8">
      {/**
      <div className="w-full flex items-center">
        <span className="mr-2">Set wide terminal</span>
        <Switch
          val={settings.wideTerminal}
          onChange={(e) => {
            dispatch(setWideTerminal({ wideTerminal: e }));
          }}
        />
      </div> 
       
       **/}
      <div className="mb-5">
        <InfoItem title="URL">
          <a
            className={twMerge(
              "flex items-center justify-center text-sm font-bold cursor-pointer",
              "text-cyan-800 hover:text-gray-800 rounded-full transition-all duration-300 shadow-2xl",
            )}
            href={`https://${item.status!.hostname}`}
            target="_blank"
          >
            {`https://${item.status!.hostname}`}
            <span className="ml-1">
              <BiLinkExternal />
            </span>
          </a>
        </InfoItem>
      </div>

      <TabGroup workspace={props.workspace} />

      <div>
        <div className="w-full min-h-[500px]">
          {tg.terminals.map((x, idx) => (
            <div
              className={twJoin(
                x.id !== tg.activeTerminal ? `hidden` : undefined,
              )}
              key={x.id}
            >
              <Terminal item={x} />
            </div>
          ))}
        </div>
      </div>
    </div>
  );
};

const Page = () => {
  const ctx = useContextWorkspace();

  return (
    <PageWrap qry={ctx.workspace}>
      {ctx.workspace.data && <TerminalGroupC workspace={ctx.workspace.data} />}
    </PageWrap>
  );
};

export default Page;
