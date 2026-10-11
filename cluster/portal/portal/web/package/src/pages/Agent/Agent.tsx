import type { AgentInfo } from "@/apis/agent/protocol";
import Meta from "@/components/Meta";
import { getClientWorkspace } from "@/utils/client";
import { Button, Loader, Menu } from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import { GetOptions, ObjectReference } from "@octelium/apis/main/metav1";
import type { RpcError } from "@protobuf-ts/runtime-rpc";
import {
  IconAlertCircle,
  IconCheck,
  IconChevronDown,
  IconExternalLink,
  IconFileText,
  IconFingerprint,
  IconPower,
  IconRefresh,
  IconSelector,
  IconShieldCheck,
  IconSparkles,
  IconTerminal2,
} from "@tabler/icons-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { motion, MotionConfig } from "framer-motion";
import * as React from "react";
import { toast } from "react-hot-toast";
import { Link } from "react-router-dom";
import { twMerge } from "tailwind-merge";
import Chat from "./Chat";
import { AgentClient } from "./client";
import { AgentMark, Shimmer, StatusDot, type DotTone } from "./ui";
import {
  getWorkspaceDisplayName,
  getWorkspaceFailure,
  getWorkspacePath,
  isWorkspaceStarting,
  isWorkspaceStopping,
  workspaceStateLabel,
} from "./utils";

const devAgentURL = import.meta.env.VITE_CORDIUM_AGENT_URL as
  string | undefined;

const S = WsPB.Workspace_Status_State;

const userConfigKey = ["agent", "userConfig"];
const agentStartupHintMs = 90000;
const ease = [0.22, 1, 0.36, 1] as const;

const getErrorMessage = (err: unknown): string =>
  (err as RpcError | Error | undefined)?.message ?? String(err);

const isNotFound = (err: unknown) =>
  (err as RpcError | undefined)?.code === "NOT_FOUND";

const workspaceTone = (ws?: WsPB.Workspace): DotTone => {
  if (getWorkspaceFailure(ws)) return "red";
  if (ws?.status?.state === S.RUNNING) return "green";
  if (isWorkspaceStarting(ws) || isWorkspaceStopping(ws)) return "blue";
  return "gray";
};

const Shell = (props: { children: React.ReactNode }) => (
  <div className="relative flex h-full min-h-[480px] w-full overflow-hidden bg-surface">
    {props.children}
  </div>
);

const StatePanel = (props: {
  title: string;
  icon?: React.ReactNode;
  tone?: "default" | "error";
  loading?: boolean;
  header?: React.ReactNode;
  children?: React.ReactNode;
}) => (
  <div className="flex min-w-0 flex-1 flex-col">
    <Meta title="Agent" />
    <div className="flex h-12 shrink-0 items-center gap-2.5 px-3">
      <AgentMark />
      <span className="flex-1 text-[0.84rem] font-bold text-ink">Agent</span>
      {props.header}
    </div>
    <div className="flex min-h-0 flex-1 overflow-y-auto px-4 py-8">
      <motion.div
        key={props.title}
        initial={{ opacity: 0, y: 8 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.3, ease }}
        className="m-auto flex w-full max-w-md flex-col items-center text-center"
      >
        {props.icon !== undefined ? (
          <span
            className={twMerge(
              "flex h-11 w-11 items-center justify-center rounded-2xl shadow-panel",
              props.tone === "error"
                ? "bg-hue-rose-soft text-hue-rose ring-1 ring-hue-rose-line"
                : "bg-inverted text-on-inverted",
            )}
          >
            {props.icon}
          </span>
        ) : (
          <AgentMark size="lg" live={props.loading} />
        )}
        <h2 className="mt-4 text-base font-bold text-ink">{props.title}</h2>
        <div className="mt-1.5 w-full text-xs leading-5 text-ink-muted">
          {props.children}
        </div>
      </motion.div>
    </div>
  </div>
);

const bootSteps: { label: string; states: WsPB.Workspace_Status_State[] }[] = [
  {
    label: "Starting the Workspace",
    states: [S.STOPPED, S.INIT_REQUEST, S.INITIALIZING],
  },
  {
    label: "Preparing the image",
    states: [S.PULLING_IMAGE, S.BUILDING_IMAGE],
  },
  {
    label: "Starting the runtime",
    states: [S.STARTING_RUNTIME, S.PREPARING],
  },
  {
    label: "Starting the agent",
    states: [S.RUNNING],
  },
];

const BootSteps = (props: { state: WsPB.Workspace_Status_State }) => {
  const current = Math.max(
    0,
    bootSteps.findIndex((step) => step.states.includes(props.state)),
  );

  return (
    <ol className="mx-auto mt-5 w-full max-w-[280px] text-left">
      {bootSteps.map((step, idx) => {
        const done = idx < current;
        const active = idx === current;
        return (
          <li key={step.label} className="relative flex gap-3 pb-4 last:pb-0">
            {idx < bootSteps.length - 1 && (
              <span
                className={twMerge(
                  "absolute left-[9.5px] top-6 h-[calc(100%-20px)] w-px transition-colors duration-500",
                  done ? "bg-hue-emerald-line" : "bg-line",
                )}
              />
            )}
            <span
              className={twMerge(
                "relative z-[1] flex h-5 w-5 shrink-0 items-center justify-center rounded-full transition-colors duration-300",
                done
                  ? "bg-hue-emerald-solid text-white"
                  : active
                    ? "bg-surface ring-1 ring-line-strong"
                    : "bg-surface ring-1 ring-line",
              )}
            >
              {done ? (
                <IconCheck size={11} stroke={3} />
              ) : active ? (
                <Loader size={10} color="gray" />
              ) : (
                <span className="h-1.5 w-1.5 rounded-full bg-ink-faint" />
              )}
            </span>
            <span className="min-w-0 pt-px">
              <span
                className={twMerge(
                  "block text-xs font-semibold",
                  done
                    ? "text-ink-body"
                    : active
                      ? "text-ink"
                      : "text-ink-subtle",
                )}
              >
                {active ? <Shimmer>{step.label}</Shimmer> : step.label}
              </span>
              {active && idx < bootSteps.length - 1 && (
                <span className="block text-[0.7rem]/4 text-ink-muted">
                  {workspaceStateLabel(props.state)}
                </span>
              )}
            </span>
          </li>
        );
      })}
    </ol>
  );
};

const AgentFooter = (props: { info: AgentInfo }) => (
  <div className="flex items-center gap-2.5 px-2 py-1.5">
    <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-line bg-surface text-ink-soft shadow-panel">
      <IconTerminal2 size={15} />
    </span>
    <span className="min-w-0 flex-1">
      <span className="block truncate text-xs font-semibold text-ink-strong">
        {props.info.workspace?.name ?? "Local agent"}
      </span>
      <span className="block truncate text-[0.7rem]/4 text-ink-muted">
        {props.info.name} {props.info.version}
      </span>
    </span>
  </div>
);

const AgentReady = (props: {
  url: string;
  info?: AgentInfo;
  footer?: React.ReactNode;
  header?: React.ReactNode;
}) => {
  const client = React.useMemo(() => new AgentClient(props.url), [props.url]);
  const infoQuery = useQuery({
    queryKey: ["agent", client.baseUrl, "info"],
    queryFn: () => client.info(),
    initialData: props.info,
  });

  if (!infoQuery.data) {
    return (
      <StatePanel title="Connecting to the agent" loading header={props.header}>
        <p>Reaching the agent inside its Workspace…</p>
      </StatePanel>
    );
  }

  return (
    <Chat
      client={client}
      info={infoQuery.data}
      footer={props.footer ?? <AgentFooter info={infoQuery.data} />}
    />
  );
};

const WorkspaceMenu = (props: {
  ws: WsPB.Workspace;
  compact?: boolean;
  busy?: boolean;
  onRestart: () => void;
  onStop: () => void;
}) => {
  const { ws } = props;
  const name = ws.metadata?.name ?? "";
  const state = ws.status?.state ?? S.UNKNOWN;
  const running = state === S.RUNNING;
  const tone = workspaceTone(ws);

  return (
    <Menu
      position={props.compact ? "bottom-end" : "top-start"}
      width={props.compact ? 272 : "target"}
      withinPortal
    >
      <Menu.Target>
        {props.compact ? (
          <button
            type="button"
            aria-label="Agent Workspace"
            className="flex h-8 min-w-0 cursor-pointer items-center gap-2 rounded-lg border border-line bg-surface px-2.5 text-ink-body shadow-panel transition-[border-color,box-shadow] duration-150 hover:border-line-strong hover:shadow-raised"
          >
            {props.busy ? (
              <Loader size={10} color="gray" />
            ) : (
              <StatusDot tone={tone} pulse={tone === "blue"} />
            )}
            <span className="max-w-[160px] truncate text-xs font-semibold">
              {getWorkspaceDisplayName(ws)}
            </span>
            <IconChevronDown size={13} className="shrink-0 text-ink-subtle" />
          </button>
        ) : (
          <button
            type="button"
            aria-label="Agent Workspace"
            className="group flex w-full cursor-pointer items-center gap-2.5 rounded-lg px-2 py-1.5 text-left outline-none transition-colors duration-150 hover:bg-surface-strong/50 focus-visible:ring-2 focus-visible:ring-focus"
          >
            <span className="relative flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-line bg-surface text-ink-soft shadow-panel">
              {props.busy ? (
                <Loader size={12} color="gray" />
              ) : (
                <IconTerminal2 size={15} />
              )}
              <span className="absolute -bottom-0.5 -right-0.5 flex rounded-full bg-surface-subtle p-[2px]">
                <StatusDot tone={tone} pulse={tone === "blue"} />
              </span>
            </span>
            <span className="min-w-0 flex-1">
              <span className="block truncate text-xs font-semibold text-ink-strong">
                {getWorkspaceDisplayName(ws)}
              </span>
              <span className="block truncate text-[0.7rem]/4 text-ink-muted">
                {workspaceStateLabel(state)} · {name}
              </span>
            </span>
            <IconSelector size={14} className="shrink-0 text-ink-subtle" />
          </button>
        )}
      </Menu.Target>
      <Menu.Dropdown>
        <Menu.Label>
          <span className="block truncate">
            {getWorkspaceDisplayName(ws)} · {name}
          </span>
        </Menu.Label>
        <Menu.Item
          component={Link}
          to={getWorkspacePath(name)}
          leftSection={<IconExternalLink size={13} />}
        >
          Open the Workspace
        </Menu.Item>
        <Menu.Item
          component={Link}
          to={getWorkspacePath(name, "terminals")}
          leftSection={<IconTerminal2 size={13} />}
        >
          Open a terminal
        </Menu.Item>
        <Menu.Item
          component={Link}
          to={getWorkspacePath(name, "logs")}
          leftSection={<IconFileText size={13} />}
        >
          View logs
        </Menu.Item>
        <Menu.Divider />
        <Menu.Item
          leftSection={<IconRefresh size={13} />}
          disabled={!running || props.busy}
          onClick={props.onRestart}
        >
          Update and restart
        </Menu.Item>
        <Menu.Item
          leftSection={<IconPower size={13} />}
          disabled={(!running && !isWorkspaceStarting(ws)) || props.busy}
          onClick={props.onStop}
        >
          Stop
        </Menu.Item>
      </Menu.Dropdown>
    </Menu>
  );
};

const WorkspaceRunner = (props: {
  workspaceRef: ObjectReference;
  onGone: () => void;
}) => {
  const queryClient = useQueryClient();
  const uid = props.workspaceRef.uid;
  const [userStopped, setUserStopped] = React.useState(false);
  const [slowURL, setSlowURL] = React.useState<string>();
  const startedRef = React.useRef(false);
  const wsKey = ["agent", "workspace", uid];

  const wsQuery = useQuery({
    queryKey: wsKey,
    queryFn: async () =>
      (await getClientWorkspace().getWorkspace(GetOptions.create({ uid })))
        .response,
    retry: (count, err) => !isNotFound(err) && count < 2,
    refetchInterval: (query) => {
      const state = query.state.data?.status?.state;
      if (query.state.error) return false;
      if (state === S.RUNNING) return 30000;
      if (state === S.STOPPED && !restarting) return false;
      return 2000;
    },
  });

  const ws = wsQuery.data;
  const state = ws?.status?.state ?? S.UNKNOWN;
  const stopped = state === S.STOPPED;
  const running = state === S.RUNNING;
  const hostname = ws?.status?.hostname;

  const refresh = () => queryClient.invalidateQueries({ queryKey: wsKey });

  const getRef = () =>
    ObjectReference.create({ uid, name: ws?.metadata?.name });

  const startAgent = async () => {
    try {
      const { response } = await getClientWorkspace().initializeAgent(
        WsPB.InitializeAgentRequest.create({}),
      );
      if (response.status?.agentWorkspaceRef?.uid !== uid) {
        queryClient.setQueryData(userConfigKey, response);
        return;
      }
    } catch (err) {
      if ((err as RpcError).code === "FAILED_PRECONDITION") throw err;
    }
    try {
      await getClientWorkspace().startWorkspace(
        WsPB.StartWorkspaceRequest.create({ workspaceRef: getRef() }),
      );
    } catch (err) {
      if ((err as RpcError).code !== "ALREADY_EXISTS") throw err;
    }
  };

  const start = useMutation({
    mutationFn: startAgent,
    onSettled: () => refresh(),
    onError: (err) => toast.error(getErrorMessage(err)),
  });

  const stop = useMutation({
    mutationFn: async () =>
      getClientWorkspace().stopWorkspace(
        WsPB.StopWorkspaceRequest.create({ workspaceRef: getRef() }),
      ),
    onSettled: () => refresh(),
    onError: (err) => toast.error(getErrorMessage(err)),
  });

  const restart = useMutation({
    mutationFn: async () => {
      await getClientWorkspace().stopWorkspace(
        WsPB.StopWorkspaceRequest.create({ workspaceRef: getRef() }),
      );
      const deadline = Date.now() + 5 * 60 * 1000;
      for (;;) {
        const { response } = await getClientWorkspace().getWorkspace(
          GetOptions.create({ uid }),
        );
        queryClient.setQueryData(wsKey, response);
        if (response.status?.state === S.STOPPED) break;
        if (Date.now() > deadline) {
          throw new Error("The agent Workspace did not stop in time");
        }
        await new Promise((resolve) => window.setTimeout(resolve, 2000));
      }
      await startAgent();
    },
    onSettled: () => refresh(),
    onError: (err) => toast.error(getErrorMessage(err)),
  });
  const restarting = restart.isPending;

  React.useEffect(() => {
    if (!stopped || startedRef.current || userStopped || restarting) return;
    startedRef.current = true;
    start.mutate();
  }, [stopped, userStopped, restarting, start]);

  const url = running && hostname ? `https://${hostname}` : undefined;

  const infoQuery = useQuery({
    queryKey: ["agent", url, "info"],
    queryFn: () => new AgentClient(url!).info(AbortSignal.timeout(8000)),
    enabled: !!url,
    retry: false,
    refetchInterval: (query) => (query.state.data ? false : 2500),
  });
  const infoReady = !!infoQuery.data;

  React.useEffect(() => {
    if (!url || infoReady) return;
    const timer = window.setTimeout(() => setSlowURL(url), agentStartupHintMs);
    return () => window.clearTimeout(timer);
  }, [url, infoReady]);

  if (wsQuery.isPending) {
    return (
      <StatePanel title="Loading your agent" loading>
        <p>Looking up your agent Workspace…</p>
      </StatePanel>
    );
  }

  if (wsQuery.isError || !ws) {
    const gone = isNotFound(wsQuery.error);
    return (
      <StatePanel
        title={
          gone
            ? "Your agent Workspace does not exist anymore"
            : "Could not get your agent Workspace"
        }
        tone="error"
        icon={<IconAlertCircle size={21} />}
      >
        <p>
          {gone
            ? "A new Workspace is created for the agent. Your previous conversations were stored in the deleted Workspace."
            : getErrorMessage(wsQuery.error)}
        </p>
        {gone ? (
          <Button
            className="mt-4"
            size="xs"
            leftSection={<IconSparkles size={13} />}
            onClick={props.onGone}
          >
            Set up the agent again
          </Button>
        ) : (
          <Button
            className="mt-4"
            size="xs"
            variant="default"
            leftSection={<IconRefresh size={13} />}
            onClick={() => void wsQuery.refetch()}
          >
            Retry
          </Button>
        )}
      </StatePanel>
    );
  }

  const name = ws.metadata?.name ?? "";
  const logsPath = getWorkspacePath(name, "logs");
  const failure = getWorkspaceFailure(ws);

  const menu = (compact: boolean) => (
    <WorkspaceMenu
      ws={ws}
      compact={compact}
      busy={start.isPending || stop.isPending || restarting}
      onStop={() => {
        setUserStopped(true);
        stop.mutate();
      }}
      onRestart={() => restart.mutate()}
    />
  );

  if (url && infoQuery.data && !restarting) {
    return (
      <AgentReady
        url={url}
        info={infoQuery.data}
        footer={menu(false)}
        header={menu(true)}
      />
    );
  }

  if (running && !restarting) {
    return (
      <StatePanel title="Starting the agent" loading header={menu(true)}>
        <p>
          The Workspace is running and the agent is starting. The first start
          can take a few minutes while its dependencies are installed.
        </p>
        <BootSteps state={state} />
        {slowURL === url && (
          <p className="mt-5 rounded-lg border border-hue-amber-line bg-hue-amber-soft px-3 py-2 text-hue-amber">
            The agent is taking longer than expected.{" "}
            <Link
              className="font-semibold underline underline-offset-2"
              to={logsPath}
            >
              Check the Workspace logs
            </Link>
            {infoQuery.error ? ` (${getErrorMessage(infoQuery.error)})` : ""}
          </p>
        )}
      </StatePanel>
    );
  }

  if (isWorkspaceStarting(ws) || (stopped && (start.isPending || restarting))) {
    return (
      <StatePanel
        title="Starting your agent Workspace"
        loading
        header={menu(true)}
      >
        <p>Your conversations and files are kept between restarts.</p>
        <BootSteps state={state} />
      </StatePanel>
    );
  }

  if (isWorkspaceStopping(ws) || restarting) {
    return (
      <StatePanel
        title={
          restarting
            ? "Restarting your agent Workspace"
            : "Stopping your agent Workspace"
        }
        loading
        header={menu(true)}
      >
        <p>
          {restarting
            ? "The Workspace is restarted with the latest agent configuration."
            : "Your conversations and files are kept while it is stopped."}
        </p>
      </StatePanel>
    );
  }

  return (
    <StatePanel
      title={
        failure ? "The Workspace failed to start" : "Your agent Workspace is stopped"
      }
      tone={failure ? "error" : "default"}
      icon={failure ? <IconAlertCircle size={21} /> : <IconPower size={19} />}
      header={menu(true)}
    >
      {failure && (
        <p className="mb-2 break-words rounded-lg border border-hue-rose-line bg-hue-rose-soft px-3 py-2 text-left font-mono text-[0.7rem]/4 text-hue-rose">
          {failure}
        </p>
      )}
      <p>
        Your conversations and files are kept while the Workspace is stopped.
      </p>
      <div className="mt-4 flex justify-center gap-2">
        <Button
          size="xs"
          leftSection={<IconPower size={13} />}
          loading={start.isPending}
          onClick={() => {
            setUserStopped(false);
            start.mutate();
          }}
        >
          {failure ? "Try again" : "Resume"}
        </Button>
        <Button
          size="xs"
          variant="default"
          leftSection={<IconFileText size={13} />}
          component={Link}
          to={logsPath}
        >
          View logs
        </Button>
      </div>
    </StatePanel>
  );
};

const setupPoints = [
  {
    icon: IconTerminal2,
    text: "Runs inside a dedicated Workspace in your personal octelium Space",
  },
  {
    icon: IconFingerprint,
    text: "Acts on your behalf with your own identity and permissions",
  },
  {
    icon: IconShieldCheck,
    text: "Asks for your approval before risky changes, as you configure it",
  },
];

const AgentEnvironment = () => {
  const queryClient = useQueryClient();

  const userConfigQuery = useQuery({
    queryKey: userConfigKey,
    queryFn: async () =>
      (
        await getClientWorkspace().getUserConfig(
          WsPB.GetUserConfigRequest.create({}),
        )
      ).response,
    retry: 1,
    refetchOnWindowFocus: false,
  });

  const initialize = useMutation({
    mutationFn: async () =>
      (
        await getClientWorkspace().initializeAgent(
          WsPB.InitializeAgentRequest.create({}),
        )
      ).response,
    onSuccess: (data) => queryClient.setQueryData(userConfigKey, data),
    onError: (err) => toast.error(getErrorMessage(err)),
  });

  if (userConfigQuery.isPending) {
    return (
      <StatePanel title="Loading your agent" loading>
        <p>Looking up your agent environment…</p>
      </StatePanel>
    );
  }

  if (userConfigQuery.isError) {
    return (
      <StatePanel
        title="The agent is not available"
        tone="error"
        icon={<IconAlertCircle size={21} />}
      >
        <p>{getErrorMessage(userConfigQuery.error)}</p>
        <Button
          className="mt-4"
          size="xs"
          variant="default"
          leftSection={<IconRefresh size={13} />}
          onClick={() => void userConfigQuery.refetch()}
        >
          Retry
        </Button>
      </StatePanel>
    );
  }

  const workspaceRef = userConfigQuery.data.status?.agentWorkspaceRef;
  if (!workspaceRef?.uid) {
    return (
      <StatePanel title="Set up your agent" icon={<IconSparkles size={19} />}>
        <p>
          An AI agent that creates and runs your Workspaces, executes commands
          and tests inside them, moves files between them and reaches the
          Octelium Services that you can access. You can use your own Claude or
          ChatGPT subscription.
        </p>
        <ul className="mt-5 space-y-2 text-left">
          {setupPoints.map((point) => (
            <li
              key={point.text}
              className="flex items-center gap-3 rounded-xl border border-line bg-surface-subtle px-3 py-2.5"
            >
              <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg border border-line bg-surface text-ink-soft">
                <point.icon size={14} />
              </span>
              <span className="text-xs leading-5 text-ink-body">
                {point.text}
              </span>
            </li>
          ))}
        </ul>
        <Button
          className="mt-5"
          size="sm"
          leftSection={<IconSparkles size={14} />}
          loading={initialize.isPending}
          onClick={() => initialize.mutate()}
        >
          Set up my agent
        </Button>
      </StatePanel>
    );
  }

  return (
    <WorkspaceRunner
      key={workspaceRef.uid}
      workspaceRef={workspaceRef}
      onGone={() => initialize.mutate()}
    />
  );
};

const Agent = () => (
  <MotionConfig reducedMotion="user">
    <Shell>
      {devAgentURL ? <AgentReady url={devAgentURL} /> : <AgentEnvironment />}
    </Shell>
  </MotionConfig>
);

export default Agent;
