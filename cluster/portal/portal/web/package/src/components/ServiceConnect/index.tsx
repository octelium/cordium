import { CopyButton, StatusDot, type DotTone } from "@/pages/Agent/ui";
import {
  getErrorMessage,
  getWorkspaceDisplayName,
  getWorkspacePath,
  workspaceStateLabel,
} from "@/pages/Agent/utils";
import { getDomain } from "@/utils";
import { useAppSelector } from "@/utils/hooks";
import { getServiceHostname } from "@/utils/octelium";
import { ActionIcon, Button, Drawer, Loader, Tooltip } from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import { Service } from "@octelium/apis/main/userv1";
import {
  IconAlertCircle,
  IconAlertTriangle,
  IconBox,
  IconCheck,
  IconChevronDown,
  IconExternalLink,
  IconFileText,
  IconInfoCircle,
  IconMinus,
  IconPower,
  IconRefresh,
  IconSparkles,
  IconTerminal2,
  IconX,
} from "@tabler/icons-react";
import { AnimatePresence, motion } from "framer-motion";
import * as React from "react";
import { Link, useNavigate } from "react-router-dom";
import { twMerge } from "tailwind-merge";
import {
  buildConnectPlan,
  mainCommand,
  toolLabels,
  type ConnectPlan,
} from "./plan";
import {
  ConnectSession,
  planSteps,
  SessionStore,
  type SessionState,
  type SessionStep,
} from "./session";
import { useAgentWorkspace } from "./workspace";

const ease = [0.22, 1, 0.36, 1] as const;

const emptyLog: string[] = [];

const consoleAccent = "var(--color-zinc-400)";

const useSession = (key: string | undefined, create: () => ConnectSession) => {
  const [store] = React.useState(() => new SessionStore());

  React.useEffect(() => {
    store.retain();
    return () => store.scheduleDispose();
  }, [store]);

  const session = key ? store.ensure(key, create) : undefined;
  const previous = React.useRef<string | undefined>(undefined);

  React.useEffect(() => {
    const last = previous.current;
    previous.current = key;
    if (last && last !== key) store.release(last);
  }, [store, key]);

  return session;
};

const noSession = () => () => {};

const useSessionState = (session?: ConnectSession) =>
  React.useSyncExternalStore(session?.subscribe ?? noSession, () =>
    session?.getSnapshot(),
  );

const toolStepLabel = (step: SessionStep) => {
  if (!(step.id in toolLabels)) return step.label;
  switch (step.status) {
    case "running":
      return `Installing ${step.label}`;
    case "done":
      return step.detail === "Installed"
        ? `Installed ${step.label}`
        : `${step.label} is available`;
    case "failed":
      return `${step.label} could not be installed`;
    case "skipped":
      return `${step.label} was skipped`;
    default:
      return `Check ${step.label}`;
  }
};

const StepIcon = (props: { status: SessionStep["status"] }) => (
  <span
    className={twMerge(
      "flex h-[18px] w-[18px] shrink-0 items-center justify-center rounded-full transition-colors duration-300",
      props.status === "done" && "bg-hue-emerald-solid text-console",
      props.status === "failed" && "bg-hue-rose-solid text-console",
      props.status === "running" && "ring-1 ring-zinc-700",
      (props.status === "pending" || props.status === "skipped") &&
        "text-zinc-400 ring-1 ring-zinc-700",
    )}
  >
    {props.status === "done" ? (
      <IconCheck size={11} stroke={3} />
    ) : props.status === "failed" ? (
      <IconX size={11} stroke={3} />
    ) : props.status === "running" ? (
      <Loader size={10} color={consoleAccent} />
    ) : props.status === "skipped" ? (
      <IconMinus size={10} stroke={3} />
    ) : (
      <span className="h-1 w-1 rounded-full bg-zinc-400" />
    )}
  </span>
);

const StepList = (props: { steps: SessionStep[] }) => (
  <ol className="flex flex-col gap-2" aria-label="Connection steps">
    {props.steps.map((step) => (
      <li
        key={step.id}
        data-step={step.id}
        data-status={step.status}
        className="flex min-w-0 items-center gap-2.5"
      >
        <StepIcon status={step.status} />
        <span
          className={twMerge(
            "min-w-0 flex-1 truncate text-xs font-semibold",
            step.status === "pending" || step.status === "skipped"
              ? "text-zinc-400"
              : step.status === "failed"
                ? "text-rose-400"
                : "text-zinc-200",
          )}
        >
          {toolStepLabel(step)}
        </span>
        {step.detail && step.status !== "done" && (
          <span
            className="max-w-[45%] shrink-0 truncate text-[0.7rem]/4 text-zinc-400"
            title={step.detail}
          >
            {step.detail}
          </span>
        )}
      </li>
    ))}
  </ol>
);

export const CanvasCard = (props: {
  title: string;
  subtitle?: React.ReactNode;
  tone?: "default" | "error";
  icon?: React.ReactNode;
  children?: React.ReactNode;
  footer?: React.ReactNode;
}) => (
  <motion.div
    initial={{ opacity: 0, y: 6 }}
    animate={{ opacity: 1, y: 0 }}
    exit={{ opacity: 0, y: 4 }}
    transition={{ duration: 0.25, ease }}
    className="absolute inset-0 z-10 flex overflow-y-auto bg-console/80 p-4 backdrop-blur-[2px]"
  >
    <div
      role="status"
      className="m-auto w-full max-w-sm rounded-xl border border-zinc-800 bg-console-chrome p-4 shadow-overlay"
    >
      <div className="flex items-start gap-3">
        <span
          className={twMerge(
            "flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border border-zinc-800 bg-zinc-800/60",
            props.tone === "error" ? "text-rose-400" : "text-zinc-100",
          )}
        >
          {props.icon ?? <IconTerminal2 size={16} />}
        </span>
        <div className="min-w-0 flex-1">
          <p className="text-sm font-bold text-zinc-100">{props.title}</p>
          {props.subtitle && (
            <p className="mt-0.5 break-words text-xs leading-5 text-zinc-400">
              {props.subtitle}
            </p>
          )}
        </div>
      </div>
      {props.children && <div className="mt-4">{props.children}</div>}
      {props.footer && (
        <div className="mt-4 flex flex-wrap items-center justify-end gap-2">
          {props.footer}
        </div>
      )}
    </div>
  </motion.div>
);

const canvasButton =
  "inline-flex h-7 cursor-pointer items-center gap-1.5 rounded-md px-2.5 !text-xs !font-semibold outline-none transition-colors duration-150 focus-visible:ring-2 focus-visible:ring-zinc-400 disabled:cursor-default disabled:opacity-50";

export const CanvasButton = (props: {
  children: React.ReactNode;
  onClick?: () => void;
  to?: string;
  primary?: boolean;
  disabled?: boolean;
}) => {
  const className = twMerge(
    canvasButton,
    props.primary
      ? "bg-zinc-100 text-zinc-900 hover:bg-white"
      : "text-zinc-300 hover:bg-zinc-800",
  );
  return props.to ? (
    <Link className={className} to={props.to}>
      {props.children}
    </Link>
  ) : (
    <button
      type="button"
      className={className}
      disabled={props.disabled}
      onClick={props.onClick}
    >
      {props.children}
    </button>
  );
};

export const CanvasSpinner = () => <Loader size={14} color={consoleAccent} />;

const ProgressCard = (props: {
  service: Service;
  plan: ConnectPlan;
  workspace?: WsPB.Workspace;
  steps: SessionStep[];
  log?: string[];
  onSkip?: () => void;
}) => {
  const [details, setDetails] = React.useState(false);
  const log = props.log ?? emptyLog;
  const logRef = React.useRef<HTMLPreElement>(null);

  React.useEffect(() => {
    const element = logRef.current;
    if (element) element.scrollTop = element.scrollHeight;
  }, [log, details]);

  return (
    <CanvasCard
      title={`Connecting to ${getServiceHostname(props.service)}`}
      subtitle={
        <>
          {props.plan.tool ? (
            <span className="font-mono text-zinc-200">{props.plan.label}</span>
          ) : (
            "A shell"
          )}{" "}
          in your agent Workspace{" "}
          <span className="font-semibold text-zinc-200">
            {getWorkspaceDisplayName(props.workspace)}
          </span>
        </>
      }
      footer={
        (log.length > 0 || props.onSkip) && (
          <>
            {log.length > 0 && (
              <CanvasButton onClick={() => setDetails((value) => !value)}>
                <IconChevronDown
                  size={13}
                  className={twMerge(
                    "transition-transform duration-150",
                    details && "rotate-180",
                  )}
                />
                {details ? "Hide details" : "Show details"}
              </CanvasButton>
            )}
            <span className="flex-1" />
            {props.onSkip && (
              <CanvasButton onClick={props.onSkip}>
                Skip and open the shell
              </CanvasButton>
            )}
          </>
        )
      }
    >
      <StepList steps={props.steps} />
      {details && log.length > 0 && (
        <pre
          ref={logRef}
          aria-label="Setup output"
          className="mt-3 max-h-40 overflow-auto whitespace-pre-wrap break-all rounded-lg border border-zinc-800 bg-console p-2 font-mono text-[0.7rem]/4 text-zinc-400"
        >
          {log.join("\n")}
        </pre>
      )}
    </CanvasCard>
  );
};

export interface ConnectStatus {
  tone: DotTone;
  label: string;
  pulse?: boolean;
}

const StatusPill = (props: ConnectStatus) => (
  <span
    role="status"
    className="flex shrink-0 items-center gap-1.5 rounded-full border border-line bg-surface px-1.5 py-1 text-[0.7rem]/none font-semibold text-ink-soft sm:px-2"
  >
    <StatusDot tone={props.tone} pulse={props.pulse} />
    <span className="max-sm:sr-only">{props.label}</span>
  </span>
);

export const ConnectHeader = (props: {
  title: string;
  status?: ConnectStatus;
  command?: string;
  commandLabel?: string;
  actions?: React.ReactNode;
}) => {
  const { command } = props;
  return (
    <Drawer.Header className="!min-h-0 !gap-3 !border-b !border-line !bg-surface-subtle !px-3 !py-2.5 sm:!px-4">
      <span className="hidden h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-inverted text-on-inverted sm:flex">
        <IconTerminal2 size={16} />
      </span>
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-2">
          <Drawer.Title className="min-w-0 truncate !text-sm/5 !font-bold !text-ink-strong">
            {props.title}
          </Drawer.Title>
          {props.status && <StatusPill {...props.status} />}
        </div>
        {command && (
          <div className="flex min-h-[22px] min-w-0 items-center gap-0.5">
            <span
              className="min-w-0 truncate font-mono text-[0.72rem]/4 text-ink-muted"
              title={command}
              data-testid="connect-command"
            >
              {command}
            </span>
            <CopyButton value={() => command} label={props.commandLabel} />
          </div>
        )}
      </div>
      <div className="flex shrink-0 items-center gap-1.5">
        {props.actions}
        <Drawer.CloseButton
          data-autofocus
          aria-label="Close"
          className="!ml-0 !text-ink-soft"
        />
      </div>
    </Drawer.Header>
  );
};

const Strip = (props: {
  tone: "info" | "warning" | "error";
  children: React.ReactNode;
  onDismiss?: () => void;
}) => {
  const Icon =
    props.tone === "info"
      ? IconInfoCircle
      : props.tone === "warning"
        ? IconAlertTriangle
        : IconAlertCircle;
  return (
    <div
      role={props.tone === "info" ? "note" : "alert"}
      className={twMerge(
        "flex shrink-0 items-start gap-2 border-b px-3 py-2 text-xs leading-5 sm:px-4",
        props.tone === "info" && "border-line bg-surface-subtle text-ink-soft",
        props.tone === "warning" &&
          "border-hue-amber-line bg-hue-amber-soft text-hue-amber",
        props.tone === "error" &&
          "border-hue-rose-line bg-hue-rose-soft text-hue-rose",
      )}
    >
      <Icon size={14} className="mt-[3px] shrink-0" />
      <div className="min-w-0 flex-1 break-words">{props.children}</div>
      {props.onDismiss && (
        <ActionIcon
          size="sm"
          variant="subtle"
          color="gray"
          aria-label="Dismiss"
          onClick={props.onDismiss}
        >
          <IconX size={13} />
        </ActionIcon>
      )}
    </div>
  );
};

const statusOf = (
  state: SessionState | undefined,
  env: ReturnType<typeof useAgentWorkspace>,
): ConnectStatus => {
  if (state) {
    switch (state.phase) {
      case "ready":
        return state.issue || state.warning
          ? { tone: "amber", label: "Needs attention" }
          : { tone: "green", label: "Connected" };
      case "ended":
        return { tone: "gray", label: "Ended" };
      case "error":
        return { tone: "red", label: "Failed" };
      default:
        return { tone: "blue", label: "Connecting", pulse: true };
    }
  }
  if (env.configQuery.isError || env.startError || env.failure) {
    return { tone: "red", label: "Unavailable" };
  }
  if (env.starting || env.configQuery.isPending || env.running) {
    return { tone: "blue", label: "Starting", pulse: true };
  }
  if (env.stopping) return { tone: "blue", label: "Stopping", pulse: true };
  return { tone: "gray", label: "Not connected" };
};

const EnvironmentCanvas = (props: {
  env: ReturnType<typeof useAgentWorkspace>;
  service: Service;
  plan: ConnectPlan;
}) => {
  const { env } = props;
  const ws = env.workspace;

  if (env.configQuery.isError) {
    return (
      <CanvasCard
        title="Terminals are not available"
        tone="error"
        icon={<IconAlertCircle size={17} />}
        subtitle={getErrorMessage(env.configQuery.error)}
        footer={
          <CanvasButton onClick={() => void env.configQuery.refetch()}>
            <IconRefresh size={13} />
            Retry
          </CanvasButton>
        }
      />
    );
  }

  if (env.configQuery.isPending || (env.uid && env.workspaceQuery.isPending)) {
    return (
      <CanvasCard
        title="Looking up your agent Workspace"
        icon={<CanvasSpinner />}
        subtitle="Terminals run in the Workspace of your Cordium agent."
      />
    );
  }

  if (!env.uid || env.missing) {
    return (
      <CanvasCard
        title={
          env.missing
            ? "Your agent Workspace does not exist anymore"
            : "Set up your agent"
        }
        icon={<IconSparkles size={16} />}
        subtitle={
          env.missing
            ? "A new Workspace is created for the agent to run terminals in."
            : "Terminals run in the Workspace of your Cordium agent, with your own identity and permissions. It is set up once and kept in your personal octelium Space."
        }
        footer={
          <>
            {env.initialize.error && (
              <span className="mr-auto text-[0.7rem]/4 text-rose-400">
                {getErrorMessage(env.initialize.error)}
              </span>
            )}
            <CanvasButton
              primary
              disabled={env.initialize.isPending}
              onClick={() => env.initialize.mutate()}
            >
              {env.initialize.isPending ? (
                <Loader size={10} color="var(--color-zinc-900)" />
              ) : (
                <IconSparkles size={13} />
              )}
              {env.missing ? "Set up again" : "Set up"}
            </CanvasButton>
          </>
        }
      />
    );
  }

  if (!ws) {
    return (
      <CanvasCard
        title="Could not get your agent Workspace"
        tone="error"
        icon={<IconAlertCircle size={17} />}
        subtitle={getErrorMessage(env.workspaceQuery.error)}
        footer={
          <CanvasButton onClick={() => void env.workspaceQuery.refetch()}>
            <IconRefresh size={13} />
            Retry
          </CanvasButton>
        }
      />
    );
  }

  if (env.starting || env.running) {
    return (
      <ProgressCard
        service={props.service}
        plan={props.plan}
        workspace={ws}
        steps={[
          {
            id: "workspace",
            label: "Start the agent Workspace",
            status: env.running ? "done" : "running",
            detail: env.state ? workspaceStateLabel(env.state) : undefined,
          },
          ...planSteps(props.plan),
        ]}
      />
    );
  }

  if (env.stopping) {
    return (
      <CanvasCard
        title="Your agent Workspace is stopping"
        icon={<CanvasSpinner />}
        subtitle="You can start it again once it is stopped."
      />
    );
  }

  const failed = !!env.failure || !!env.startError;

  return (
    <CanvasCard
      title={
        env.failure
          ? "Your agent Workspace failed"
          : env.startError
            ? "Your agent Workspace could not be started"
            : "Your agent Workspace is stopped"
      }
      tone={failed ? "error" : "default"}
      icon={failed ? <IconAlertCircle size={17} /> : <IconPower size={16} />}
      subtitle={
        env.startError
          ? getErrorMessage(env.startError)
          : (env.failure ??
            "Start it to open a terminal and connect to this Service.")
      }
      footer={
        <>
          <CanvasButton to={getWorkspacePath(ws.metadata?.name ?? "", "logs")}>
            <IconFileText size={13} />
            View logs
          </CanvasButton>
          <CanvasButton
            primary
            disabled={env.startPending}
            onClick={env.startWorkspace}
          >
            {env.startPending ? (
              <Loader size={10} color="var(--color-zinc-900)" />
            ) : (
              <IconPower size={13} />
            )}
            Start
          </CanvasButton>
        </>
      }
    />
  );
};

const Snippets = (props: {
  plan: ConnectPlan;
  disabled: boolean;
  onType: (text: string) => void;
}) =>
  props.plan.snippets.length > 0 ? (
    <div
      role="toolbar"
      aria-label="Quick commands"
      className="scrollbar-none flex shrink-0 items-center gap-1.5 overflow-x-auto border-b border-line bg-surface px-3 py-1.5 sm:px-4"
    >
      <span className="mr-1 shrink-0 text-[0.7rem]/4 font-semibold uppercase tracking-[0.06em] text-ink-muted">
        Quick
      </span>
      {props.plan.snippets.map((snippet) => (
        <Tooltip
          key={snippet.text}
          label={`Type ${snippet.text}`}
          withArrow
          openDelay={400}
        >
          <button
            type="button"
            disabled={props.disabled}
            onClick={() => props.onType(snippet.text)}
            className="shrink-0 cursor-pointer whitespace-nowrap rounded-md border border-line bg-surface px-2 py-0.5 !font-mono !text-[0.7rem]/4 text-ink-soft shadow-panel outline-none transition-colors duration-150 hover:border-line-strong hover:text-ink focus-visible:ring-2 focus-visible:ring-focus disabled:cursor-default disabled:opacity-50"
          >
            {snippet.label}
          </button>
        </Tooltip>
      ))}
    </div>
  ) : null;

const ServiceConnect = (props: { service: Service }) => {
  const { service } = props;
  const domain = getDomain();
  const navigate = useNavigate();
  const fontSize = useAppSelector((s) => s.settings.terminalFontSize);
  const plan = React.useMemo(
    () => buildConnectPlan(service, domain),
    [service, domain],
  );
  const env = useAgentWorkspace();
  const ws = env.workspace;
  const terminalRef = React.useRef<HTMLDivElement>(null);

  const key =
    env.running && ws?.metadata?.uid
      ? `connect:${ws.metadata.uid}:${plan.tool ?? "shell"}:${mainCommand(plan) ?? ""}`
      : undefined;
  const session = useSession(
    key,
    () => new ConnectSession({ plan, workspace: ws!, domain, fontSize }),
  );
  const state = useSessionState(session);

  React.useEffect(() => {
    const element = terminalRef.current;
    if (!session || !element) return;
    session.attach(element);
    session.start();
    return () => session.detach(element);
  }, [session]);

  React.useEffect(() => {
    session?.setFontSize(fontSize);
  }, [session, fontSize]);

  const command = mainCommand(plan, state?.facts);
  const ready = state?.phase === "ready";
  const preparing = state?.phase === "starting" || state?.phase === "preparing";
  const terminalPath =
    ws && state?.terminalId
      ? `${getWorkspacePath(ws.metadata?.name ?? "", "terminals")}?terminal=${encodeURIComponent(state.terminalId)}`
      : undefined;

  const openInWorkspace = () => {
    if (!terminalPath) return;
    session?.keep();
    navigate(terminalPath);
  };

  return (
    <>
      <ConnectHeader
        title={`Connect to ${getServiceHostname(service)}`}
        status={statusOf(state, env)}
        command={command ?? plan.address}
        commandLabel={command ? "Copy the command" : "Copy the address"}
        actions={
          <>
            <Tooltip label="Start a new session">
              <ActionIcon
                variant="default"
                size={30}
                aria-label="Start a new session"
                disabled={!session || preparing}
                onClick={() => session?.restart()}
              >
                <IconRefresh size={14} />
              </ActionIcon>
            </Tooltip>
            <Tooltip
              label="The terminal stays open after you leave this page"
              multiline
              w={220}
            >
              <Button
                visibleFrom="sm"
                variant="default"
                size="xs"
                h={30}
                leftSection={<IconExternalLink size={13} />}
                disabled={!terminalPath}
                onClick={openInWorkspace}
              >
                Open in Workspace
              </Button>
            </Tooltip>
            <Tooltip label="Open in Workspace">
              <ActionIcon
                hiddenFrom="sm"
                variant="default"
                size={30}
                aria-label="Open in Workspace"
                disabled={!terminalPath}
                onClick={openInWorkspace}
              >
                <IconExternalLink size={14} />
              </ActionIcon>
            </Tooltip>
          </>
        }
      />
      <Drawer.Body className="!flex !min-h-0 !flex-1 !flex-col !p-0">
        {state?.issue && (
          <Strip tone="error" onDismiss={() => session?.dismissIssue()}>
            The connection seems to have failed:{" "}
            <code className="break-all font-mono font-semibold">
              {state.issue}
            </code>
          </Strip>
        )}
        {state?.warning && <Strip tone="warning">{state.warning}</Strip>}
        {(plan.hint || plan.inferred) && (
          <Strip tone={plan.hint?.tone ?? "info"}>
            {plan.hint?.text}
            {plan.inferred && (
              <span className={twMerge(plan.hint && "text-ink-muted")}>
                {plan.hint && " "}
                {plan.inferred}
              </span>
            )}
          </Strip>
        )}
        <Snippets
          plan={plan}
          disabled={!ready}
          onType={(text) => session?.type(text)}
        />

        <div className="console-surface relative min-h-0 flex-1 bg-console">
          <div
            ref={terminalRef}
            data-testid="connect-terminal"
            className="absolute inset-0 py-2 pl-3 pr-1"
          />
          <AnimatePresence>
            {!session && (
              <EnvironmentCanvas
                key="environment"
                env={env}
                service={service}
                plan={plan}
              />
            )}
            {session && state && preparing && (
              <ProgressCard
                key="progress"
                service={service}
                plan={plan}
                workspace={ws}
                steps={[
                  {
                    id: "workspace",
                    label: "Start the agent Workspace",
                    status: "done",
                  },
                  ...state.steps,
                ]}
                log={state.log}
                onSkip={
                  state.phase === "preparing" ? () => session.skip() : undefined
                }
              />
            )}
            {session && state?.phase === "error" && (
              <CanvasCard
                key="error"
                title="The terminal is not available"
                tone="error"
                icon={<IconAlertCircle size={17} />}
                subtitle={state.error}
                footer={
                  <CanvasButton primary onClick={() => session.restart()}>
                    <IconRefresh size={13} />
                    Try again
                  </CanvasButton>
                }
              />
            )}
          </AnimatePresence>
          {session && state?.phase === "ended" && (
            <div className="absolute inset-x-0 bottom-4 z-10 flex justify-center px-4">
              <div
                role="status"
                className="flex items-center gap-3 rounded-lg border border-zinc-800 bg-console-chrome py-1.5 pl-3 pr-1.5 text-xs text-zinc-200 shadow-overlay"
              >
                The session has ended
                <CanvasButton primary onClick={() => session.restart()}>
                  <IconRefresh size={13} />
                  New session
                </CanvasButton>
              </div>
            </div>
          )}
        </div>

        <div className="flex min-w-0 shrink-0 items-center gap-3 border-t border-zinc-800 bg-console-chrome px-3 py-1.5 text-[0.7rem]/4 text-zinc-400">
          <span className="flex min-w-0 items-center gap-1.5">
            <IconBox size={12} className="shrink-0" />
            <span className="truncate" title={ws?.metadata?.name}>
              {ws ? getWorkspaceDisplayName(ws) : "Agent Workspace"}
            </span>
          </span>
          {state?.terminalId && (
            <span className="hidden truncate font-mono sm:inline">
              {state.terminalId}
            </span>
          )}
          <span className="flex-1" />
          {state && state.cols > 0 && (
            <span
              className="shrink-0 font-mono tabular-nums"
              data-testid="connect-size"
            >
              {state.cols}×{state.rows}
            </span>
          )}
        </div>
      </Drawer.Body>
    </>
  );
};

export default ServiceConnect;
