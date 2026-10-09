import type {
  AgentSettings,
  AuthProvider,
  LoginEvent,
  LoginSession,
  ModelInfo,
} from "@/apis/agent/protocol";
import {
  Button,
  Drawer,
  PasswordInput,
  SegmentedControl,
  Select,
  Switch,
  TextInput,
} from "@mantine/core";
import {
  IconAlertCircle,
  IconCircleCheck,
  IconExternalLink,
  IconKey,
  IconLogout,
  IconShieldCheck,
  IconSparkles,
} from "@tabler/icons-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import * as React from "react";
import { toast } from "react-hot-toast";
import { twMerge } from "tailwind-merge";
import { AgentClient } from "./client";
import { approvalModes, modelLabel } from "./utils";

const THINKING_LEVELS = [
  "off",
  "minimal",
  "low",
  "medium",
  "high",
  "xhigh",
  "max",
];

const modelKey = (model: ModelInfo) => `${model.provider}/${model.id}`;

const lastEvent = <T extends LoginEvent["type"]>(
  session: LoginSession,
  type: T,
) =>
  session.events.filter((e) => e.type === type).at(-1) as
    Extract<LoginEvent, { type: T }> | undefined;

const Step = (props: {
  n: number;
  done?: boolean;
  children: React.ReactNode;
}) => (
  <div className="flex gap-2.5">
    <span
      className={twMerge(
        "mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-full text-[0.68rem] font-bold",
        props.done
          ? "bg-hue-emerald-soft text-hue-emerald"
          : "bg-inverted text-on-inverted",
      )}
    >
      {props.done ? <IconCircleCheck size={13} /> : props.n}
    </span>
    <div className="min-w-0 flex-1">{props.children}</div>
  </div>
);

const LoginFlow = (props: {
  client: AgentClient;
  provider: AuthProvider;
  onDone: (session: LoginSession) => void;
  onCancel: () => void;
}) => {
  const { client, provider } = props;
  const [session, setSession] = React.useState<LoginSession>();
  const [error, setError] = React.useState<string>();
  const [value, setValue] = React.useState("");
  const [submitting, setSubmitting] = React.useState(false);
  const [opened, setOpened] = React.useState(false);
  const sessionRef = React.useRef<LoginSession | undefined>(undefined);
  const onDone = React.useRef(props.onDone);

  React.useEffect(() => {
    onDone.current = props.onDone;
  });

  React.useEffect(() => {
    let active = true;
    let timer: number | undefined;

    const update = (next: LoginSession) => {
      sessionRef.current = next;
      setSession(next);
      if (next.status !== "pending") {
        onDone.current(next);
        return;
      }
      timer = window.setTimeout(poll, 1000);
    };

    const poll = () => {
      const id = sessionRef.current?.id;
      if (!active || !id) return;
      client
        .getLogin(id)
        .then((next) => active && update(next))
        .catch((err: Error) => active && setError(err.message));
    };

    client
      .startLogin(provider.id)
      .then((next) => active && update(next))
      .catch((err: Error) => active && setError(err.message));

    return () => {
      active = false;
      window.clearTimeout(timer);
      const current = sessionRef.current;
      if (current?.status === "pending") {
        void client.cancelLogin(current.id).catch(() => undefined);
      }
    };
  }, [client, provider.id]);

  const prompt = session?.prompt;

  const answer = async (answerValue: string) => {
    if (!session || !prompt) return;
    setSubmitting(true);
    setError(undefined);
    try {
      const next = await client.answerLogin(session.id, prompt.id, answerValue);
      sessionRef.current = next;
      setSession(next);
      setValue("");
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setSubmitting(false);
    }
  };

  const authURL = session ? lastEvent(session, "auth_url") : undefined;
  const deviceCode = session ? lastEvent(session, "device_code") : undefined;
  const progress = session ? lastEvent(session, "progress") : undefined;

  return (
    <div className="mt-2 rounded-xl border border-line bg-surface-subtle px-3.5 py-3">
      <p className="text-[0.82rem] font-bold text-ink">
        Sign in with {provider.name}
      </p>

      {!session && !error && (
        <p className="mt-2 text-[0.78rem] text-ink-muted">
          Starting the sign-in…
        </p>
      )}

      <div className="mt-3 flex flex-col gap-3">
        {authURL && (
          <Step n={1} done={opened}>
            <Button
              component="a"
              href={authURL.url}
              target="_blank"
              rel="noreferrer noopener"
              size="xs"
              leftSection={<IconExternalLink size={13} />}
              onClick={() => setOpened(true)}
            >
              Open the sign-in page
            </Button>
            <p className="mt-1.5 text-[0.75rem] leading-5 text-ink-muted">
              Sign in with your subscription account. The tokens are stored only
              inside your agent Workspace.
            </p>
          </Step>
        )}

        {deviceCode && (
          <Step n={1}>
            <p className="text-[0.78rem] text-ink-soft">
              Open{" "}
              <a
                href={deviceCode.verificationUri}
                target="_blank"
                rel="noreferrer noopener"
                className="font-semibold text-ink underline"
              >
                {deviceCode.verificationUri}
              </a>{" "}
              and enter the code
            </p>
            <code className="mt-1 inline-block rounded-md border border-line bg-surface px-2 py-0.5 font-mono text-[0.95rem] font-bold tracking-widest text-ink">
              {deviceCode.userCode}
            </code>
          </Step>
        )}

        {prompt && prompt.type === "select" && (
          <Step n={authURL ? 2 : 1}>
            <p className="text-[0.78rem] text-ink-soft">{prompt.message}</p>
            <div className="mt-1.5 flex flex-wrap gap-1.5">
              {prompt.options?.map((option) => (
                <Button
                  key={option.id}
                  size="xs"
                  variant="default"
                  disabled={submitting}
                  onClick={() => void answer(option.id)}
                >
                  {option.label}
                </Button>
              ))}
            </div>
          </Step>
        )}

        {prompt && prompt.type !== "select" && (
          <Step n={authURL ? 2 : 1}>
            <form
              className="flex items-end gap-2"
              onSubmit={(event) => {
                event.preventDefault();
                void answer(value.trim());
              }}
            >
              {prompt.type === "secret" ? (
                <PasswordInput
                  className="flex-1"
                  size="xs"
                  label={prompt.message}
                  placeholder={prompt.placeholder}
                  value={value}
                  onChange={(event) => setValue(event.currentTarget.value)}
                />
              ) : (
                <TextInput
                  className="flex-1"
                  size="xs"
                  label={prompt.message}
                  description={
                    authURL
                      ? "Paste the code that is shown after signing in, or the full URL of the page you were redirected to, even if it fails to load."
                      : undefined
                  }
                  placeholder={prompt.placeholder}
                  value={value}
                  onChange={(event) => setValue(event.currentTarget.value)}
                />
              )}
              <Button
                type="submit"
                size="xs"
                loading={submitting}
                disabled={value.trim() === ""}
              >
                Continue
              </Button>
            </form>
          </Step>
        )}
      </div>

      {progress && session?.status === "pending" && !prompt && (
        <p className="mt-2 text-[0.78rem] text-ink-muted">{progress.message}</p>
      )}

      {session?.status === "failed" && (
        <p className="mt-2 flex items-start gap-1.5 text-[0.78rem] font-semibold text-hue-rose">
          <IconAlertCircle size={14} className="mt-0.5 shrink-0" />
          {session.error?.message ?? "The sign-in failed"}
        </p>
      )}

      {error && (
        <p className="mt-2 flex items-start gap-1.5 text-[0.78rem] font-semibold text-hue-rose">
          <IconAlertCircle size={14} className="mt-0.5 shrink-0" />
          {error}
        </p>
      )}

      <div className="mt-3 flex justify-end">
        <Button
          size="compact-xs"
          variant="subtle"
          color="gray"
          onClick={props.onCancel}
        >
          {session?.status === "pending" || !session ? "Cancel" : "Close"}
        </Button>
      </div>
    </div>
  );
};

const SectionTitle = (props: {
  icon: React.ReactNode;
  title: string;
  description?: string;
}) => (
  <div className="mb-2.5">
    <p className="flex items-center gap-1.5 text-[0.72rem] font-bold uppercase tracking-[0.08em] text-ink-muted">
      {props.icon}
      {props.title}
    </p>
    {props.description && (
      <p className="mt-1 text-[0.78rem] leading-5 text-ink-muted">
        {props.description}
      </p>
    )}
  </div>
);

const SettingsDrawer = (props: {
  client: AgentClient;
  opened: boolean;
  onClose: () => void;
}) => {
  const { client } = props;
  const queryClient = useQueryClient();
  const [signingIn, setSigningIn] = React.useState<AuthProvider>();

  const modelsKey = ["agent", client.baseUrl, "models"];
  const providersKey = ["agent", client.baseUrl, "providers"];
  const settingsKey = ["agent", client.baseUrl, "settings"];
  const infoKey = ["agent", client.baseUrl, "info"];

  const modelsQuery = useQuery({
    queryKey: modelsKey,
    queryFn: () => client.listModels(),
    enabled: props.opened,
  });

  const providersQuery = useQuery({
    queryKey: providersKey,
    queryFn: () => client.listAuthProviders(),
    enabled: props.opened,
  });

  const settingsQuery = useQuery({
    queryKey: settingsKey,
    queryFn: () => client.getSettings(),
    enabled: props.opened,
  });

  const setModel = useMutation({
    mutationFn: (req: {
      provider: string;
      id: string;
      thinkingLevel?: string;
    }) => client.setModel(req),
    onSuccess: (data) => {
      queryClient.setQueryData(modelsKey, data);
      void queryClient.invalidateQueries({ queryKey: infoKey });
    },
    onError: (err: Error) => toast.error(err.message),
  });

  const updateSettings = useMutation({
    mutationFn: (approvals: Partial<AgentSettings["approvals"]>) =>
      client.updateSettings({ approvals }),
    onSuccess: (data) => {
      queryClient.setQueryData(settingsKey, data);
      void queryClient.invalidateQueries({ queryKey: infoKey });
    },
    onError: (err: Error) => toast.error(err.message),
  });

  const logout = useMutation({
    mutationFn: (provider: string) => client.logout(provider),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: providersKey });
      void queryClient.invalidateQueries({ queryKey: modelsKey });
      void queryClient.invalidateQueries({ queryKey: infoKey });
    },
    onError: (err: Error) => toast.error(err.message),
  });

  const models = React.useMemo(
    () => modelsQuery.data?.models ?? [],
    [modelsQuery.data],
  );
  const current = modelsQuery.data?.current;
  const approvals = settingsQuery.data?.approvals;

  const groups = React.useMemo(() => {
    const ret = new Map<string, { value: string; label: string }[]>();
    for (const model of models) {
      const items = ret.get(model.provider) ?? [];
      items.push({ value: modelKey(model), label: model.name ?? model.id });
      ret.set(model.provider, items);
    }
    return [...ret.entries()].map(([group, items]) => ({ group, items }));
  }, [models]);

  return (
    <Drawer
      opened={props.opened}
      onClose={props.onClose}
      size="md"
      title={
        <span className="flex items-center gap-2 text-[0.95rem]">
          <IconSparkles size={16} />
          Agent settings
        </span>
      }
    >
      <div className="flex flex-col gap-7 pb-4">
        <section>
          <SectionTitle
            icon={<IconSparkles size={13} />}
            title="Model"
            description="The model that answers in all your conversations."
          />
          {modelsQuery.data?.error && (
            <p className="mb-2.5 flex items-start gap-1.5 rounded-lg border border-hue-amber-line bg-hue-amber-soft px-2.5 py-2 text-[0.78rem] text-hue-amber">
              <IconAlertCircle size={14} className="mt-0.5 shrink-0" />
              {modelsQuery.data.error.message}
            </p>
          )}
          <Select
            searchable
            placeholder={modelsQuery.isPending ? "Loading…" : "Choose a model"}
            nothingFoundMessage="No models are available. Sign in to a subscription first."
            data={groups}
            value={current ? modelKey(current) : null}
            disabled={setModel.isPending}
            onChange={(value) => {
              const model = models.find((m) => modelKey(m) === value);
              if (model)
                setModel.mutate({ provider: model.provider, id: model.id });
            }}
          />
          {current?.reasoning && (
            <div className="mt-3">
              <p className="mb-1.5 text-[0.78rem] font-semibold text-ink-body">
                Thinking effort
              </p>
              <SegmentedControl
                fullWidth
                size="xs"
                data={THINKING_LEVELS}
                value={modelsQuery.data?.thinkingLevel ?? "medium"}
                disabled={setModel.isPending}
                onChange={(value) =>
                  setModel.mutate({
                    provider: current.provider,
                    id: current.id,
                    thinkingLevel: value,
                  })
                }
              />
            </div>
          )}
        </section>

        <section>
          <SectionTitle
            icon={<IconShieldCheck size={13} />}
            title="Approvals"
            description="When the agent pauses for your approval. Your Cluster permissions always apply."
          />
          <div className="flex flex-col gap-1.5">
            {approvalModes.map((mode) => {
              const selected = approvals?.api === mode.value;
              return (
                <button
                  key={mode.value}
                  type="button"
                  disabled={!approvals || updateSettings.isPending}
                  onClick={() => updateSettings.mutate({ api: mode.value })}
                  className={twMerge(
                    "flex cursor-pointer items-start gap-2.5 rounded-xl border px-3 py-2.5 text-left transition-colors duration-150",
                    selected
                      ? "border-ink bg-surface-hover"
                      : "border-line bg-surface hover:border-line-strong",
                  )}
                >
                  <span
                    className={twMerge(
                      "mt-1 flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded-full border-2",
                      selected ? "border-ink" : "border-line-strong",
                    )}
                  >
                    {selected && (
                      <span className="h-1.5 w-1.5 rounded-full bg-ink" />
                    )}
                  </span>
                  <span className="min-w-0">
                    <span className="block text-[0.84rem] font-bold text-ink">
                      {mode.label}
                    </span>
                    <span className="block text-[0.75rem] leading-5 text-ink-muted">
                      {mode.description}
                    </span>
                  </span>
                </button>
              );
            })}
          </div>
          <Switch
            className="mt-3"
            label="Ask before running shell commands"
            description="In the agent's Workspace and in your other Workspaces"
            checked={approvals?.commands ?? false}
            disabled={!approvals || updateSettings.isPending}
            onChange={(event) =>
              updateSettings.mutate({ commands: event.currentTarget.checked })
            }
          />
        </section>

        <section>
          <SectionTitle
            icon={<IconKey size={13} />}
            title="Subscriptions"
            description="Use your own Claude or ChatGPT subscription. The credentials never leave your agent Workspace."
          />
          <ul className="flex flex-col gap-2">
            {(providersQuery.data?.items ?? []).map((provider) => (
              <li
                key={provider.id}
                className="flex items-center gap-3 rounded-xl border border-line bg-surface px-3 py-2.5"
              >
                <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-surface-muted text-ink-soft">
                  <IconKey size={15} />
                </span>
                <div className="min-w-0 flex-1">
                  <p className="truncate text-[0.84rem] font-bold text-ink">
                    {provider.name}
                  </p>
                  <p
                    className={twMerge(
                      "text-[0.72rem] font-semibold",
                      provider.configured
                        ? "text-hue-emerald"
                        : "text-ink-subtle",
                    )}
                  >
                    {provider.configured
                      ? provider.oauth
                        ? "Signed in"
                        : "Configured"
                      : "Not signed in"}
                  </p>
                </div>
                {provider.oauth ? (
                  <Button
                    size="xs"
                    variant="default"
                    leftSection={<IconLogout size={13} />}
                    loading={
                      logout.isPending && logout.variables === provider.id
                    }
                    onClick={() => logout.mutate(provider.id)}
                  >
                    Sign out
                  </Button>
                ) : (
                  <Button
                    size="xs"
                    disabled={!!signingIn}
                    onClick={() => setSigningIn(provider)}
                  >
                    {provider.loginLabel ?? "Sign in"}
                  </Button>
                )}
              </li>
            ))}
          </ul>
          {providersQuery.data?.items.length === 0 && (
            <p className="text-[0.78rem] text-ink-muted">
              No subscription providers are enabled for this agent.
            </p>
          )}

          {signingIn && (
            <LoginFlow
              key={signingIn.id}
              client={client}
              provider={signingIn}
              onCancel={() => setSigningIn(undefined)}
              onDone={(session) => {
                void queryClient.invalidateQueries({ queryKey: providersKey });
                void queryClient.invalidateQueries({ queryKey: modelsKey });
                void queryClient.invalidateQueries({ queryKey: infoKey });
                if (session.status === "completed") {
                  toast.success(
                    session.model
                      ? `Signed in. Now using ${modelLabel(session.model)}`
                      : "Signed in",
                  );
                  setSigningIn(undefined);
                }
              }}
            />
          )}
        </section>
      </div>
    </Drawer>
  );
};

export default SettingsDrawer;
