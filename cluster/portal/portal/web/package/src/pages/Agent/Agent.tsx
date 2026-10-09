import type { AgentInfo } from "@/apis/agent/protocol";
import Meta from "@/components/Meta";
import StateBadge from "@/components/StateBadge";
import { getClientWorkspace } from "@/utils/client";
import { ActionIcon, Button, Loader, Menu } from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import { GetOptions, ObjectReference } from "@octelium/apis/main/metav1";
import type { RpcError } from "@protobuf-ts/runtime-rpc";
import {
  IconAlertTriangle,
  IconCheck,
  IconDots,
  IconExternalLink,
  IconFileText,
  IconLoader2,
  IconPlayerPlay,
  IconPlayerStop,
  IconRefresh,
  IconSparkles,
  IconTerminal2,
} from "@tabler/icons-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { motion } from "framer-motion";
import * as React from "react";
import { toast } from "react-hot-toast";
import { Link } from "react-router-dom";
import { twMerge } from "tailwind-merge";
import Chat from "./Chat";
import { AgentClient } from "./client";
import {
  getWorkspaceFailure,
  isWorkspaceStarting,
  isWorkspaceStopping,
} from "./utils";

const devAgentURL = import.meta.env.VITE_CORDIUM_AGENT_URL as
  string | undefined;

const S = WsPB.Workspace_Status_State;

const userConfigKey = ["agent", "userConfig"];
const agentStartupHintMs = 90000;

const getErrorMessage = (err: unknown): string =>
  (err as RpcError | Error | undefined)?.message ?? String(err);

const isNotFound = (err: unknown) =>
  (err as RpcError | undefined)?.code === "NOT_FOUND";

const Shell = (props: { children: React.ReactNode }) => (
  <div className="flex h-full min-h-0 flex-col bg-surface">
    <header className="flex h-[52px] shrink-0 items-center gap-2.5 border-b border-line px-4">
      <span className="flex h-7 w-7 items-center justify-center rounded-lg bg-inverted text-on-inverted">
        <IconSparkles size={14} />
      </span>
      <span className="text-[0.9rem] font-bold text-ink">Cordium Agent</span>
    </header>
    <div className="flex min-h-0 flex-1 items-start justify-center overflow-y-auto px-4 py-10 md:items-center md:py-6">
      {props.children}
    </div>
  </div>
);

const Card = (props: {
  icon?: React.ReactNode;
  title: string;
  tone?: "default" | "error";
  children?: React.ReactNode;
  actions?: React.ReactNode;
}) => (
  <motion.div
    initial={{ opacity: 0, y: 10 }}
    animate={{ opacity: 1, y: 0 }}
    transition={{ duration: 0.25, ease: "easeOut" }}
    className="w-full max-w-lg rounded-2xl border border-line bg-surface px-7 py-7 text-center shadow-panel"
  >
    <span
      className={twMerge(
        "mx-auto flex h-12 w-12 items-center justify-center rounded-2xl",
        props.tone === "error"
          ? "bg-hue-rose-soft text-hue-rose"
          : "bg-inverted text-on-inverted",
      )}
    >
      {props.icon ?? <IconSparkles size={22} />}
    </span>
    <h2 className="mt-4 text-lg font-bold tracking-tight text-ink">
      {props.title}
    </h2>
    <div className="mt-2 text-[0.84rem] leading-6 text-ink-muted">
      {props.children}
    </div>
    {props.actions && (
      <div className="mt-5 flex flex-wrap justify-center gap-2">
        {props.actions}
      </div>
    )}
  </motion.div>
);

const startupSteps: { label: string; states: WsPB.Workspace_Status_State[] }[] =
  [
    { label: "Requested", states: [S.INIT_REQUEST, S.INITIALIZING] },
    {
      label: "Preparing the image",
      states: [S.PULLING_IMAGE, S.BUILDING_IMAGE],
    },
    { label: "Starting the runtime", states: [S.STARTING_RUNTIME] },
    { label: "Running the setup tasks", states: [S.PREPARING] },
    { label: "Starting the agent", states: [S.RUNNING] },
  ];

const StartupProgress = (props: {
  state: WsPB.Workspace_Status_State;
  agentStarting?: boolean;
}) => {
  const current = startupSteps.findIndex((step) =>
    step.states.includes(props.state),
  );
  const active = props.agentStarting ? startupSteps.length - 1 : current;

  return (
    <ol className="mx-auto mt-5 flex max-w-xs flex-col gap-2.5 text-left">
      {startupSteps.map((step, idx) => {
        const done = idx < active;
        const isActive = idx === active;
        return (
          <li key={step.label} className="flex items-center gap-2.5">
            <span
              className={twMerge(
                "flex h-5 w-5 shrink-0 items-center justify-center rounded-full border text-[0.66rem] font-bold",
                done &&
                  "border-hue-emerald-line bg-hue-emerald-soft text-hue-emerald",
                isActive && "border-ink bg-surface text-ink",
                !done && !isActive && "border-line text-ink-faint",
              )}
            >
              {done ? (
                <IconCheck size={12} />
              ) : isActive ? (
                <IconLoader2 size={12} className="animate-spin" />
              ) : (
                idx + 1
              )}
            </span>
            <span
              className={twMerge(
                "text-[0.82rem] font-semibold",
                done && "text-ink-muted",
                isActive && "text-ink",
                !done && !isActive && "text-ink-subtle",
              )}
            >
              {step.label}
            </span>
          </li>
        );
      })}
    </ol>
  );
};

const WorkspaceCard = (props: {
  ws: WsPB.Workspace;
  onRestart: () => void;
  onStop: () => void;
  busy: boolean;
}) => {
  const { ws } = props;
  const name = ws.metadata?.name ?? "";
  const running = ws.status?.state === S.RUNNING;

  return (
    <div className="flex items-center gap-2.5 rounded-xl border border-line bg-surface px-2.5 py-2">
      <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-surface-muted text-ink-soft">
        <IconTerminal2 size={15} />
      </span>
      <div className="min-w-0 flex-1">
        <Link
          to={`/workspaces/${encodeURIComponent(name)}`}
          className="block truncate font-mono text-[0.8rem] font-bold text-ink hover:underline"
        >
          {name}
        </Link>
        <p className="truncate text-[0.7rem] font-medium text-ink-subtle">
          Agent Workspace
        </p>
      </div>
      <StateBadge state={ws.status?.state ?? S.UNKNOWN} />
      <Menu position="top-end" withinPortal width={200}>
        <Menu.Target>
          <ActionIcon
            variant="subtle"
            color="gray"
            aria-label="Agent Workspace actions"
            loading={props.busy}
          >
            <IconDots size={15} />
          </ActionIcon>
        </Menu.Target>
        <Menu.Dropdown>
          <Menu.Label>Workspace {name}</Menu.Label>
          <Menu.Item
            component={Link}
            to={`/workspaces/${encodeURIComponent(name)}`}
            leftSection={<IconExternalLink size={13} />}
          >
            Open the Workspace
          </Menu.Item>
          <Menu.Item
            component={Link}
            to={`/workspaces/${encodeURIComponent(name)}/terminals`}
            leftSection={<IconTerminal2 size={13} />}
          >
            Open a terminal
          </Menu.Item>
          <Menu.Item
            component={Link}
            to={`/workspaces/${encodeURIComponent(name)}/logs`}
            leftSection={<IconFileText size={13} />}
          >
            View the logs
          </Menu.Item>
          <Menu.Divider />
          <Menu.Item
            leftSection={<IconRefresh size={13} />}
            disabled={!running}
            onClick={props.onRestart}
          >
            Update and restart
          </Menu.Item>
          <Menu.Item
            leftSection={<IconPlayerStop size={13} />}
            disabled={!running && !isWorkspaceStarting(ws)}
            onClick={props.onStop}
          >
            Stop the agent
          </Menu.Item>
        </Menu.Dropdown>
      </Menu>
    </div>
  );
};

const AgentReady = (props: {
  url: string;
  info?: AgentInfo;
  workspace?: React.ReactNode;
}) => {
  const client = React.useMemo(() => new AgentClient(props.url), [props.url]);
  const infoQuery = useQuery({
    queryKey: ["agent", client.baseUrl, "info"],
    queryFn: () => client.info(),
    initialData: props.info,
  });

  if (!infoQuery.data) {
    return (
      <Shell>
        <Card title="Connecting to the agent">
          <Loader size="sm" color="gray" className="mx-auto my-2" />
        </Card>
      </Shell>
    );
  }

  return (
    <Chat client={client} info={infoQuery.data} workspace={props.workspace} />
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
      <Shell>
        <Card title="Loading your agent">
          <Loader size="sm" color="gray" className="mx-auto my-2" />
        </Card>
      </Shell>
    );
  }

  if (wsQuery.isError || !ws) {
    return (
      <Shell>
        <Card
          title={
            isNotFound(wsQuery.error)
              ? "Your agent Workspace does not exist anymore"
              : "Could not get your agent Workspace"
          }
          tone="error"
          icon={<IconAlertTriangle size={22} />}
          actions={
            isNotFound(wsQuery.error) ? (
              <Button
                leftSection={<IconSparkles size={15} />}
                onClick={props.onGone}
              >
                Set up the agent again
              </Button>
            ) : (
              <Button
                variant="default"
                leftSection={<IconRefresh size={15} />}
                onClick={() => void wsQuery.refetch()}
              >
                Retry
              </Button>
            )
          }
        >
          {isNotFound(wsQuery.error)
            ? "A new Workspace is created for the agent. Your previous conversations were stored in the deleted Workspace."
            : getErrorMessage(wsQuery.error)}
        </Card>
      </Shell>
    );
  }

  const name = ws.metadata?.name ?? "";
  const logsLink = `/workspaces/${encodeURIComponent(name)}/logs`;
  const workspaceCard = (
    <WorkspaceCard
      ws={ws}
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
      <AgentReady url={url} info={infoQuery.data} workspace={workspaceCard} />
    );
  }

  if (running && !restarting) {
    const waitingTooLong = slowURL === url;
    return (
      <Shell>
        <Card
          title="Starting the agent"
          actions={
            waitingTooLong && (
              <Button
                component={Link}
                to={logsLink}
                variant="default"
                leftSection={<IconFileText size={15} />}
              >
                View the Workspace logs
              </Button>
            )
          }
        >
          The Workspace <span className="font-mono font-semibold">{name}</span>{" "}
          is running and the agent is starting. The first start can take a few
          minutes while its dependencies are installed.
          <StartupProgress state={state} agentStarting />
          {waitingTooLong && (
            <p className="mt-4 text-hue-amber">
              The agent is taking longer than expected
              {infoQuery.error ? ` (${getErrorMessage(infoQuery.error)})` : ""}.
            </p>
          )}
        </Card>
      </Shell>
    );
  }

  if (isWorkspaceStarting(ws) || (stopped && (start.isPending || restarting))) {
    return (
      <Shell>
        <Card title="Starting your agent Workspace">
          Your agent runs in its own Workspace{" "}
          <span className="font-mono font-semibold">{name}</span>.
          <StartupProgress state={stopped ? S.INIT_REQUEST : state} />
        </Card>
      </Shell>
    );
  }

  if (isWorkspaceStopping(ws) || restarting) {
    return (
      <Shell>
        <Card
          title={restarting ? "Restarting the agent" : "Stopping the agent"}
        >
          <Loader size="sm" color="gray" className="mx-auto my-2" />
        </Card>
      </Shell>
    );
  }

  const failure = getWorkspaceFailure(ws);
  return (
    <Shell>
      <Card
        title={
          failure
            ? "The agent Workspace failed to start"
            : "The agent is stopped"
        }
        tone={failure ? "error" : "default"}
        icon={failure ? <IconAlertTriangle size={22} /> : undefined}
        actions={
          <>
            <Button
              leftSection={<IconPlayerPlay size={15} />}
              loading={start.isPending}
              onClick={() => {
                setUserStopped(false);
                start.mutate();
              }}
            >
              {failure ? "Try again" : "Start the agent"}
            </Button>
            <Button
              component={Link}
              to={logsLink}
              variant="default"
              leftSection={<IconFileText size={15} />}
            >
              View the logs
            </Button>
          </>
        }
      >
        {failure && (
          <p className="mb-2 font-semibold text-hue-rose">{failure}</p>
        )}
        Your conversations and files are kept in the Workspace{" "}
        <span className="font-mono font-semibold">{name}</span> while it is
        stopped.
      </Card>
    </Shell>
  );
};

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
      <Shell>
        <Card title="Loading your agent">
          <Loader size="sm" color="gray" className="mx-auto my-2" />
        </Card>
      </Shell>
    );
  }

  if (userConfigQuery.isError) {
    return (
      <Shell>
        <Card
          title="The agent is not available"
          tone="error"
          icon={<IconAlertTriangle size={22} />}
          actions={
            <Button
              variant="default"
              leftSection={<IconRefresh size={15} />}
              onClick={() => void userConfigQuery.refetch()}
            >
              Retry
            </Button>
          }
        >
          {getErrorMessage(userConfigQuery.error)}
        </Card>
      </Shell>
    );
  }

  const workspaceRef = userConfigQuery.data.status?.agentWorkspaceRef;
  if (!workspaceRef?.uid) {
    return (
      <Shell>
        <Card
          title="Meet your Cordium Agent"
          actions={
            <Button
              size="md"
              leftSection={<IconSparkles size={16} />}
              loading={initialize.isPending}
              onClick={() => initialize.mutate()}
            >
              Set up my agent
            </Button>
          }
        >
          <p>
            An AI agent that manages your Workspaces for you: it creates and
            runs them, executes commands and tests inside them, moves files
            between them and reaches the Octelium Services that you can access.
          </p>
          <p className="mt-2">
            It runs inside a dedicated Workspace of your personal{" "}
            <span className="font-mono font-semibold">octelium</span> Space and
            acts with your own identity and permissions. You can use your own
            Claude or ChatGPT subscription.
          </p>
        </Card>
      </Shell>
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
  <>
    <Meta title="Agent" />
    {devAgentURL ? <AgentReady url={devAgentURL} /> : <AgentEnvironment />}
  </>
);

export default Agent;
