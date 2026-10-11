import type {
  AgentInfo,
  AgentSettings,
  AuthProvider,
  LoginEvent,
  LoginSession,
  ModelInfo,
} from "@/apis/agent/protocol";
import Tag from "@/components/Tag";
import {
  Button,
  Combobox,
  Drawer,
  Loader,
  PasswordInput,
  Select,
  Switch,
  TextInput,
  useCombobox,
} from "@mantine/core";
import {
  IconAlertCircle,
  IconBrain,
  IconCheck,
  IconChevronDown,
  IconCircleCheck,
  IconExternalLink,
  IconKey,
  IconPhoto,
  IconSettings,
  IconShieldCheck,
  IconSparkles,
} from "@tabler/icons-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import * as React from "react";
import { toast } from "react-hot-toast";
import { twMerge } from "tailwind-merge";
import { AgentClient } from "./client";
import {
  approvalModes,
  formatCount,
  matchesTerms,
  modelLabel,
  searchTerms,
} from "./utils";

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
    <div className="mt-3 rounded-xl border border-line bg-surface-subtle px-3 py-3">
      <p className="text-xs font-semibold text-ink-body">
        Signing in with {provider.name}
      </p>

      {!session && !error && (
        <p className="mt-2 text-xs text-ink-muted">Starting the sign-in…</p>
      )}

      {authURL && (
        <div className="mt-2.5">
          <Button
            component="a"
            href={authURL.url}
            target="_blank"
            rel="noreferrer noopener"
            size="xs"
            leftSection={<IconExternalLink size={13} />}
          >
            Open the sign-in page
          </Button>
          <p className="mt-1.5 text-xs leading-5 text-ink-soft">
            Sign in on the opened page. Then copy the code that is shown to you,
            or the full URL of the page you were redirected to even if it fails
            to load, and paste it below.
          </p>
        </div>
      )}

      {deviceCode && (
        <div className="mt-2.5 text-xs leading-6 text-ink-soft">
          Open{" "}
          <a
            href={deviceCode.verificationUri}
            target="_blank"
            rel="noreferrer noopener"
            className="font-semibold text-ink underline decoration-line-strong underline-offset-2"
          >
            {deviceCode.verificationUri}
          </a>{" "}
          and enter the code{" "}
          <code className="rounded-md border border-line bg-surface px-1.5 py-0.5 font-mono font-bold tracking-widest text-ink">
            {deviceCode.userCode}
          </code>
        </div>
      )}

      {prompt && prompt.type === "select" && (
        <div className="mt-2.5">
          <p className="text-xs text-ink-soft">{prompt.message}</p>
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
        </div>
      )}

      {prompt && prompt.type !== "select" && (
        <form
          className="mt-2.5 flex items-end gap-2"
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
            Submit
          </Button>
        </form>
      )}

      {progress && session?.status === "pending" && !prompt && (
        <p className="mt-2 text-xs text-ink-muted">{progress.message}</p>
      )}

      {session?.status === "failed" && (
        <p className="mt-2 flex items-start gap-1.5 text-xs font-semibold text-hue-rose">
          <IconAlertCircle size={14} className="mt-px shrink-0" />
          {session.error?.message ?? "The sign-in failed"}
        </p>
      )}

      {error && (
        <p className="mt-2 flex items-start gap-1.5 text-xs font-semibold text-hue-rose">
          <IconAlertCircle size={14} className="mt-px shrink-0" />
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

const useModels = (client: AgentClient, enabled: boolean) => {
  const queryClient = useQueryClient();
  const modelsKey = ["agent", client.baseUrl, "models"];

  const query = useQuery({
    queryKey: modelsKey,
    queryFn: () => client.listModels(),
    enabled,
  });

  const mutation = useMutation({
    mutationFn: (req: {
      provider: string;
      id: string;
      thinkingLevel?: string;
    }) => client.setModel(req),
    onSuccess: (data) => {
      queryClient.setQueryData(modelsKey, data);
      void queryClient.invalidateQueries({
        queryKey: ["agent", client.baseUrl, "info"],
      });
    },
    onError: (err: Error) => toast.error(err.message),
  });

  return { query, mutation };
};

const ModelOption = (props: { model: ModelInfo; selected: boolean }) => {
  const { model } = props;
  return (
    <div className="flex items-center gap-2.5">
      <div className="min-w-0 flex-1">
        <p className="truncate text-[0.82rem]/5 font-semibold">
          {model.name ?? model.id}
        </p>
        <p className="flex items-center gap-1.5 truncate text-[0.7rem]/4 opacity-70">
          <span className="truncate font-mono">{model.id}</span>
          {model.contextWindow ? (
            <span className="shrink-0">
              · {formatCount(model.contextWindow)}
            </span>
          ) : null}
        </p>
      </div>
      {model.reasoning && (
        <IconBrain
          size={13}
          className="shrink-0 opacity-60"
          aria-label="Reasoning"
        />
      )}
      {model.input?.includes("image") && (
        <IconPhoto
          size={13}
          className="shrink-0 opacity-60"
          aria-label="Images"
        />
      )}
      <IconCheck
        size={14}
        stroke={2.75}
        className={twMerge("shrink-0", props.selected ? "" : "invisible")}
      />
    </div>
  );
};

export const ModelPicker = (props: {
  client: AgentClient;
  info: AgentInfo;
  onManage: () => void;
}) => {
  const { client, info } = props;
  const [query, setQuery] = React.useState("");
  const { query: modelsQuery, mutation } = useModels(
    client,
    info.capabilities.models,
  );
  const combobox = useCombobox({
    onDropdownClose: () => {
      combobox.resetSelectedOption();
      setQuery("");
    },
    onDropdownOpen: () => combobox.focusSearchInput(),
  });

  const models = React.useMemo(
    () => modelsQuery.data?.models ?? [],
    [modelsQuery.data],
  );
  const current = modelsQuery.data?.current ?? info.model;
  const currentKey = current ? modelKey(current) : undefined;
  const canManage = info.capabilities.login || info.capabilities.settings;

  const groups = React.useMemo(() => {
    const terms = searchTerms(query);
    const ret = new Map<string, ModelInfo[]>();
    for (const model of models) {
      if (
        terms.length > 0 &&
        !matchesTerms(
          `${model.provider} ${model.id} ${model.name ?? ""}`,
          terms,
        )
      ) {
        continue;
      }
      ret.set(model.provider, [...(ret.get(model.provider) ?? []), model]);
    }
    return [...ret.entries()];
  }, [models, query]);

  if (!info.capabilities.models) {
    return (
      <span className="flex h-8 min-w-0 items-center gap-1.5 px-2 text-xs font-semibold text-ink-muted">
        <IconSparkles size={13} className="shrink-0" />
        <span className="truncate">{modelLabel(current)}</span>
      </span>
    );
  }

  return (
    <Combobox
      store={combobox}
      width={320}
      position="top-start"
      offset={8}
      shadow="md"
      radius="lg"
      withinPortal
      transitionProps={{ transition: "pop", duration: 150 }}
      onOptionSubmit={(value) => {
        const model = models.find((m) => modelKey(m) === value);
        if (model && value !== currentKey) {
          mutation.mutate({ provider: model.provider, id: model.id });
        }
        combobox.closeDropdown();
      }}
      styles={{
        dropdown: {
          border: "1px solid var(--app-line)",
          boxShadow: "var(--app-shadow-overlay)",
          padding: 0,
          overflow: "hidden",
          maxWidth: "calc(100vw - 24px)",
        },
      }}
    >
      <Combobox.Target withAriaAttributes={false}>
        <button
          type="button"
          aria-label="Choose the model"
          aria-haspopup="listbox"
          aria-expanded={combobox.dropdownOpened}
          onClick={() => combobox.toggleDropdown()}
          className={twMerge(
            "flex h-8 min-w-0 max-w-[220px] cursor-pointer items-center gap-1.5 rounded-full px-2.5 transition-colors duration-150 hover:bg-surface-muted hover:text-ink",
            current ? "text-ink-soft" : "text-hue-amber",
            combobox.dropdownOpened && "bg-surface-muted text-ink",
          )}
        >
          {mutation.isPending ? (
            <Loader size={11} color="gray" />
          ) : (
            <IconSparkles size={13} className="shrink-0" />
          )}
          <span className="truncate text-xs font-semibold">
            {current ? modelLabel(current) : "Choose a model"}
          </span>
          <IconChevronDown size={12} stroke={2.5} className="shrink-0" />
        </button>
      </Combobox.Target>

      <Combobox.Dropdown>
        <div className="border-b border-line-subtle p-2">
          <Combobox.Search
            value={query}
            onChange={(event) => setQuery(event.currentTarget.value)}
            placeholder="Search models"
            styles={{
              input: {
                border: "none",
                background: "var(--app-surface-muted)",
                borderRadius: "8px",
                minHeight: "34px",
                margin: 0,
                width: "100%",
              },
            }}
          />
        </div>
        {modelsQuery.data?.error && (
          <p className="mx-2 mt-2 flex items-start gap-1.5 rounded-lg border border-hue-amber-line bg-hue-amber-soft px-2.5 py-2 text-[0.7rem]/4 text-hue-amber">
            <IconAlertCircle size={13} className="mt-px shrink-0" />
            {modelsQuery.data.error.message}
          </p>
        )}
        <Combobox.Options className="max-h-[280px] overflow-y-auto p-1.5">
          {modelsQuery.isPending ? (
            <div className="flex justify-center py-6">
              <Loader size="xs" color="gray" />
            </div>
          ) : groups.length === 0 ? (
            <Combobox.Empty className="px-3 py-6 text-xs text-ink-muted">
              {models.length === 0
                ? "No available models. Sign in to a subscription first."
                : "No models match your search"}
            </Combobox.Empty>
          ) : (
            groups.map(([provider, items]) => (
              <Combobox.Group key={provider} label={provider}>
                {items.map((model) => (
                  <Combobox.Option
                    key={modelKey(model)}
                    value={modelKey(model)}
                    active={modelKey(model) === currentKey}
                  >
                    <ModelOption
                      model={model}
                      selected={modelKey(model) === currentKey}
                    />
                  </Combobox.Option>
                ))}
              </Combobox.Group>
            ))
          )}
        </Combobox.Options>
        {(current?.reasoning || canManage) && (
          <div className="flex items-center gap-2 border-t border-line-subtle bg-surface-subtle px-3 py-2">
            {current?.reasoning && (
              <Select
                size="xs"
                className="w-32"
                aria-label="Thinking level"
                leftSection={<IconBrain size={12} />}
                data={THINKING_LEVELS}
                value={modelsQuery.data?.thinkingLevel ?? null}
                disabled={mutation.isPending}
                allowDeselect={false}
                comboboxProps={{ withinPortal: false }}
                onChange={(value) =>
                  value &&
                  mutation.mutate({
                    provider: current.provider,
                    id: current.id,
                    thinkingLevel: value,
                  })
                }
              />
            )}
            {canManage && (
              <Button
                size="compact-xs"
                variant="subtle"
                color="gray"
                className="ml-auto"
                leftSection={<IconSettings size={12} />}
                onClick={() => {
                  combobox.closeDropdown();
                  props.onManage();
                }}
              >
                Settings
              </Button>
            )}
          </div>
        )}
      </Combobox.Dropdown>
    </Combobox>
  );
};

const SectionTitle = (props: {
  icon: React.ReactNode;
  title: string;
  description: string;
}) => (
  <div className="mb-3">
    <p className="flex items-center gap-1.5 text-[0.7rem] font-bold uppercase tracking-[0.08em] text-ink-muted">
      {props.icon}
      {props.title}
    </p>
    <p className="mt-1 text-xs leading-5 text-ink-muted">
      {props.description}
    </p>
  </div>
);

const SettingsDrawer = (props: {
  client: AgentClient;
  info: AgentInfo;
  opened: boolean;
  onClose: () => void;
}) => {
  const { client, info } = props;
  const queryClient = useQueryClient();
  const [signingIn, setSigningIn] = React.useState<AuthProvider>();

  const modelsKey = ["agent", client.baseUrl, "models"];
  const providersKey = ["agent", client.baseUrl, "providers"];
  const settingsKey = ["agent", client.baseUrl, "settings"];
  const infoKey = ["agent", client.baseUrl, "info"];

  const settingsQuery = useQuery({
    queryKey: settingsKey,
    queryFn: () => client.getSettings(),
    enabled: props.opened && info.capabilities.settings,
    initialData: info.settings,
  });

  const providersQuery = useQuery({
    queryKey: providersKey,
    queryFn: () => client.listAuthProviders(),
    enabled: props.opened && info.capabilities.login,
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

  const approvals = settingsQuery.data?.approvals;

  return (
    <Drawer
      opened={props.opened}
      onClose={props.onClose}
      position="right"
      size="md"
      overlayProps={{ backgroundOpacity: 0.25, blur: 1 }}
      title={
        <span className="flex items-center gap-2 text-sm font-bold text-ink-strong">
          <IconSettings size={16} />
          Agent settings
        </span>
      }
    >
      <div className="flex flex-col gap-7 pb-4">
        {info.capabilities.settings && (
          <section>
            <SectionTitle
              icon={<IconShieldCheck size={13} />}
              title="Approvals"
              description="When the agent pauses for your approval. Your Cluster permissions always apply."
            />
            <div className="flex flex-col gap-1.5" role="radiogroup">
              {approvalModes.map((mode) => {
                const selected = approvals?.api === mode.value;
                return (
                  <button
                    key={mode.value}
                    type="button"
                    role="radio"
                    aria-checked={selected}
                    disabled={!approvals || updateSettings.isPending}
                    onClick={() => updateSettings.mutate({ api: mode.value })}
                    className={twMerge(
                      "flex cursor-pointer items-start gap-3 rounded-xl border px-3 py-2.5 text-left shadow-panel outline-none transition-[border-color,background-color] duration-150 focus-visible:ring-2 focus-visible:ring-focus disabled:cursor-default",
                      selected
                        ? "border-line-strong bg-surface-subtle"
                        : "border-line bg-surface hover:border-line-strong",
                    )}
                  >
                    <span
                      className={twMerge(
                        "mt-[3px] flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded-full border-2",
                        selected ? "border-ink" : "border-line-strong",
                      )}
                    >
                      {selected && (
                        <span className="h-1.5 w-1.5 rounded-full bg-ink" />
                      )}
                    </span>
                    <span className="min-w-0">
                      <span className="block text-[0.82rem]/5 font-semibold text-ink-strong">
                        {mode.label}
                      </span>
                      <span className="block text-xs leading-5 text-ink-muted">
                        {mode.description}
                      </span>
                    </span>
                  </button>
                );
              })}
            </div>
            <Switch
              className="mt-4"
              size="sm"
              label="Ask before running shell commands"
              description="In the agent's Workspace and in your other Workspaces"
              checked={approvals?.commands ?? false}
              disabled={!approvals || updateSettings.isPending}
              onChange={(event) =>
                updateSettings.mutate({ commands: event.currentTarget.checked })
              }
            />
          </section>
        )}

        {info.capabilities.login && (
          <section>
            <SectionTitle
              icon={<IconKey size={13} />}
              title="Subscriptions"
              description="Use your own Claude or ChatGPT subscription. The credentials are stored only inside your agent Workspace and never leave it."
            />
            <ul className="flex flex-col gap-2">
              {(providersQuery.data?.items ?? []).map((provider) => (
                <li
                  key={provider.id}
                  className="flex items-center gap-3 rounded-xl border border-line bg-surface px-3 py-2.5 shadow-panel"
                >
                  <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-line bg-surface-subtle text-ink-muted">
                    <IconKey size={14} />
                  </span>
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-[0.82rem]/5 font-semibold text-ink-strong">
                      {provider.name}
                    </p>
                    {provider.configured ? (
                      <Tag
                        tone="success"
                        icon={<IconCircleCheck size={11} />}
                        className="px-1.5 text-[0.68rem] leading-4"
                      >
                        {provider.oauth ? "Signed in" : "Configured"}
                      </Tag>
                    ) : (
                      <Tag className="px-1.5 text-[0.68rem] leading-4">
                        Not signed in
                      </Tag>
                    )}
                  </div>
                  {provider.oauth ? (
                    <Button
                      size="xs"
                      variant="default"
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
            {providersQuery.isPending && (
              <div className="flex justify-center py-4">
                <Loader size="xs" color="gray" />
              </div>
            )}
            {providersQuery.data?.items.length === 0 && (
              <p className="text-xs text-ink-muted">
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
                  void queryClient.invalidateQueries({
                    queryKey: providersKey,
                  });
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
        )}
      </div>
    </Drawer>
  );
};

export default SettingsDrawer;
