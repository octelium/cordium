import { Anchor, Breadcrumbs } from "@mantine/core";
import { IconChevronRight } from "@tabler/icons-react";
import * as React from "react";
import { Link } from "react-router-dom";

export interface Crumb {
  label: string;
  to?: string;
}

const PageHeader = (props: {
  title: React.ReactNode;
  description?: React.ReactNode;
  crumbs?: Crumb[];
  badges?: React.ReactNode;
  actions?: React.ReactNode;
}) => (
  <div className="mb-6">
    {props.crumbs && props.crumbs.length > 0 && (
      <Breadcrumbs
        separator={<IconChevronRight size={13} className="text-slate-300" />}
        separatorMargin={6}
        className="mb-2"
      >
        {props.crumbs.map((c, idx) =>
          c.to ? (
            <Anchor
              key={idx}
              component={Link}
              to={c.to}
              underline="hover"
              className="text-[0.78rem] font-semibold text-slate-500 hover:text-slate-800"
            >
              {c.label}
            </Anchor>
          ) : (
            <span
              key={idx}
              className="text-[0.78rem] font-semibold text-slate-400"
            >
              {c.label}
            </span>
          ),
        )}
      </Breadcrumbs>
    )}

    <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
          <h1 className="text-2xl font-bold tracking-tight text-slate-900 truncate">
            {props.title}
          </h1>
          {props.badges}
        </div>
        {props.description && (
          <p className="mt-1.5 text-sm font-medium text-slate-500 max-w-3xl">
            {props.description}
          </p>
        )}
      </div>

      {props.actions && (
        <div className="flex flex-wrap items-center gap-2 shrink-0">
          {props.actions}
        </div>
      )}
    </div>
  </div>
);

export default PageHeader;
