import * as React from "react";
import ReactMarkdown, { type Components } from "react-markdown";
import { Link } from "react-router-dom";
import remarkGfm from "remark-gfm";
import { CopyButton } from "./ui";
import { getResourceURIRoute, urlTransform } from "./utils";

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

const CodeBlock = (props: { language?: string; code: string }) => (
  <div className="console-surface my-3 overflow-hidden rounded-xl border border-zinc-800 bg-console">
    <div className="flex items-center justify-between border-b border-zinc-800 py-1 pl-3 pr-1.5">
      <span className="text-[0.7rem]/4 font-semibold uppercase tracking-[0.08em] text-zinc-500">
        {props.language || "code"}
      </span>
      <CopyButton dark value={() => props.code} label="Copy code" />
    </div>
    <pre className="max-h-[480px] overflow-auto px-3.5 py-3">
      <code className="font-mono text-[0.78rem] leading-6 text-zinc-100">
        {props.code}
      </code>
    </pre>
  </div>
);

const components: Components = {
  a: (props) => <AgentLink href={props.href}>{props.children}</AgentLink>,
  p: (props) => <p className="my-2 first:mt-0 last:mb-0">{props.children}</p>,
  strong: (props) => (
    <strong className="font-bold text-ink">{props.children}</strong>
  ),
  em: (props) => <em className="italic">{props.children}</em>,
  del: (props) => (
    <del className="text-ink-muted line-through">{props.children}</del>
  ),
  h1: (props) => (
    <h1 className="mb-2 mt-4 text-base font-bold text-ink first:mt-0">
      {props.children}
    </h1>
  ),
  h2: (props) => (
    <h2 className="mb-2 mt-4 text-[0.9rem] font-bold text-ink first:mt-0">
      {props.children}
    </h2>
  ),
  h3: (props) => (
    <h3 className="mb-1.5 mt-3 font-bold text-ink-strong first:mt-0">
      {props.children}
    </h3>
  ),
  h4: (props) => (
    <h4 className="mb-1 mt-3 font-bold text-ink-strong first:mt-0">
      {props.children}
    </h4>
  ),
  ul: (props) => (
    <ul className="my-2 list-disc space-y-1 pl-5 marker:text-ink-faint">
      {props.children}
    </ul>
  ),
  ol: (props) => (
    <ol className="my-2 list-decimal space-y-1 pl-5 marker:font-semibold marker:text-ink-subtle">
      {props.children}
    </ol>
  ),
  li: (props) => <li className="pl-0.5">{props.children}</li>,
  blockquote: (props) => (
    <blockquote className="my-3 border-l-2 border-line-strong bg-surface-subtle py-1 pl-3 text-ink-soft">
      {props.children}
    </blockquote>
  ),
  hr: () => <hr className="my-4 border-line" />,
  img: (props) =>
    typeof props.src === "string" ? (
      <img
        src={props.src}
        alt={props.alt ?? ""}
        loading="lazy"
        className="my-3 max-h-[420px] w-auto max-w-full rounded-xl border border-line bg-surface object-contain shadow-panel"
      />
    ) : null,
  table: (props) => (
    <div className="my-3 w-full overflow-x-auto rounded-xl border border-line">
      <table className="w-full border-collapse text-[0.82rem]/5">
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
    <td className="border-b border-line-subtle px-3 py-2 align-top">
      {props.children}
    </td>
  ),
  pre: (props) => <>{props.children}</>,
  code: (props) => {
    const text = String(props.children ?? "").replace(/\n$/, "");
    const language = /language-([\w+-]+)/.exec(props.className ?? "")?.[1];

    if (language !== undefined || text.includes("\n")) {
      return <CodeBlock language={language} code={text} />;
    }

    return (
      <code className="rounded border border-line bg-surface-muted px-1 py-0.5 font-mono text-[0.9em] font-semibold text-ink-strong">
        {text}
      </code>
    );
  },
};

const AgentMarkdown = (props: { children: string }) => (
  <div className="min-w-0 break-words text-[0.82rem]/6 text-ink-body">
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
