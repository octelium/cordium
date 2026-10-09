import { ActionIcon, Tooltip } from "@mantine/core";
import { IconCheck, IconCopy } from "@tabler/icons-react";
import * as React from "react";
import ReactMarkdown, { type Components } from "react-markdown";
import { Link } from "react-router-dom";
import remarkGfm from "remark-gfm";
import { twMerge } from "tailwind-merge";
import { copyText, getResourceURIRoute, urlTransform } from "./utils";

export const CopyButton = (props: { text: string; className?: string }) => {
  const [copied, setCopied] = React.useState(false);

  return (
    <Tooltip label={copied ? "Copied" : "Copy"}>
      <ActionIcon
        size="sm"
        variant="subtle"
        color="gray"
        aria-label="Copy"
        className={props.className}
        onClick={async () => {
          if (await copyText(props.text)) {
            setCopied(true);
            window.setTimeout(() => setCopied(false), 1500);
          }
        }}
      >
        {copied ? <IconCheck size={14} /> : <IconCopy size={14} />}
      </ActionIcon>
    </Tooltip>
  );
};

const linkClassName =
  "font-semibold text-ink underline decoration-line-strong decoration-1 underline-offset-[3px] transition-colors duration-150 hover:decoration-ink";

const AgentLink = (props: { href?: string; children?: React.ReactNode }) => {
  const route = props.href ? getResourceURIRoute(props.href) : undefined;

  if (route) {
    return (
      <Link to={route} className={linkClassName}>
        {props.children}
      </Link>
    );
  }

  if (props.href?.startsWith("octelium://")) {
    return <span className="font-semibold text-ink">{props.children}</span>;
  }

  return (
    <a
      href={props.href}
      target="_blank"
      rel="noreferrer noopener"
      className={linkClassName}
    >
      {props.children}
    </a>
  );
};

const nodeText = (node: React.ReactNode): string => {
  if (typeof node === "string" || typeof node === "number") {
    return String(node);
  }
  if (Array.isArray(node)) {
    return node.map(nodeText).join("");
  }
  if (React.isValidElement(node)) {
    return nodeText((node.props as { children?: React.ReactNode }).children);
  }
  return "";
};

const CodeBlock = (props: { children?: React.ReactNode }) => {
  const child = React.Children.toArray(props.children)[0];
  const className = React.isValidElement(child)
    ? ((child.props as { className?: string }).className ?? "")
    : "";
  const language = /language-([\w+-]+)/.exec(className)?.[1];
  const text = nodeText(props.children).replace(/\n$/, "");

  return (
    <div className="group/code my-3 overflow-hidden rounded-xl border border-line bg-surface-subtle">
      <div className="flex items-center justify-between border-b border-line-subtle px-3 py-1">
        <span className="font-mono text-[0.7rem] font-semibold uppercase tracking-[0.06em] text-ink-subtle">
          {language ?? "text"}
        </span>
        <CopyButton text={text} />
      </div>
      <pre className="max-h-[420px] overflow-auto px-3.5 py-2.5">
        <code className="font-mono text-[0.8rem] leading-relaxed text-ink-body">
          {text}
        </code>
      </pre>
    </div>
  );
};

const components: Components = {
  a: (props) => <AgentLink href={props.href}>{props.children}</AgentLink>,
  p: (props) => (
    <p className="my-2 leading-7 text-ink-body first:mt-0 last:mb-0">
      {props.children}
    </p>
  ),
  h1: (props) => (
    <h3 className="mb-2 mt-5 text-lg font-bold text-ink first:mt-0">
      {props.children}
    </h3>
  ),
  h2: (props) => (
    <h4 className="mb-2 mt-5 text-base font-bold text-ink first:mt-0">
      {props.children}
    </h4>
  ),
  h3: (props) => (
    <h5 className="mb-1.5 mt-4 text-[0.95rem] font-bold text-ink first:mt-0">
      {props.children}
    </h5>
  ),
  h4: (props) => (
    <h6 className="mb-1 mt-3 text-sm font-bold text-ink-strong first:mt-0">
      {props.children}
    </h6>
  ),
  ul: (props) => (
    <ul className="my-2 list-disc space-y-1 pl-5 text-ink-body marker:text-ink-faint">
      {props.children}
    </ul>
  ),
  ol: (props) => (
    <ol className="my-2 list-decimal space-y-1 pl-5 text-ink-body marker:font-semibold marker:text-ink-subtle">
      {props.children}
    </ol>
  ),
  li: (props) => <li className="leading-7 pl-0.5">{props.children}</li>,
  blockquote: (props) => (
    <blockquote className="my-3 border-l-[3px] border-line-strong pl-3.5 text-ink-soft">
      {props.children}
    </blockquote>
  ),
  hr: () => <hr className="my-5 border-line" />,
  strong: (props) => (
    <strong className="font-bold text-ink">{props.children}</strong>
  ),
  pre: (props) => <CodeBlock>{props.children}</CodeBlock>,
  code: (props) => (
    <code
      className={twMerge(
        "rounded-md border border-line-subtle bg-surface-muted px-1.5 py-[1px] font-mono text-[0.84em] text-ink-strong",
        props.className,
      )}
    >
      {props.children}
    </code>
  ),
  table: (props) => (
    <div className="my-3 overflow-x-auto rounded-xl border border-line">
      <table className="w-full border-collapse text-[0.84rem]">
        {props.children}
      </table>
    </div>
  ),
  thead: (props) => (
    <thead className="bg-surface-subtle text-ink-soft">{props.children}</thead>
  ),
  th: (props) => (
    <th className="whitespace-nowrap border-b border-line px-3 py-2 text-left font-bold">
      {props.children}
    </th>
  ),
  td: (props) => (
    <td className="border-b border-line-subtle px-3 py-1.5 align-top text-ink-body">
      {props.children}
    </td>
  ),
};

const AgentMarkdown = (props: { children: string }) => (
  <div className="min-w-0 break-words text-[0.92rem]">
    <ReactMarkdown
      remarkPlugins={[remarkGfm]}
      urlTransform={urlTransform}
      components={components}
    >
      {props.children}
    </ReactMarkdown>
  </div>
);

export default React.memo(AgentMarkdown);
