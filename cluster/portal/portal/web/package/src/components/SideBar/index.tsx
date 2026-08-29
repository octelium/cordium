import {
  IconLayoutDashboard,
  IconKey,
  IconServer2,
  IconSettings,
  IconStack2,
  IconTerminal2,
} from "@tabler/icons-react";
import * as React from "react";
import { NavLink as RouterNavLink } from "react-router-dom";
import { twMerge } from "tailwind-merge";

interface NavItem {
  label: string;
  to: string;
  icon: React.ReactNode;
  end?: boolean;
}

const primary: NavItem[] = [
  {
    label: "Overview",
    to: "/",
    icon: <IconLayoutDashboard size={17} />,
    end: true,
  },
  { label: "Spaces", to: "/spaces", icon: <IconStack2 size={17} /> },
  { label: "Workspaces", to: "/workspaces", icon: <IconTerminal2 size={17} /> },
];

const secondary: NavItem[] = [
  { label: "Services", to: "/services", icon: <IconServer2 size={17} /> },
  { label: "Your Secrets", to: "/usersecrets", icon: <IconKey size={17} /> },
];

const Item = (props: { item: NavItem; onNavigate?: () => void }) => (
  <RouterNavLink
    to={props.item.to}
    end={props.item.end}
    onClick={props.onNavigate}
    className={({ isActive }) =>
      twMerge(
        "flex items-center gap-2.5 rounded-lg px-3 py-2 text-[0.84rem] font-semibold",
        "transition-colors duration-150",
        isActive
          ? "bg-slate-900 text-white shadow-sm"
          : "text-slate-600 hover:bg-slate-200/70 hover:text-slate-900",
      )
    }
  >
    {props.item.icon}
    <span className="truncate">{props.item.label}</span>
  </RouterNavLink>
);

const SectionLabel = (props: { children: React.ReactNode }) => (
  <div className="px-3 pb-1.5 pt-4 text-[0.66rem] font-bold uppercase tracking-[0.09em] text-slate-400">
    {props.children}
  </div>
);

const SideBar = (props: { onNavigate?: () => void }) => (
  <nav className="flex h-full flex-col gap-1">
    {primary.map((item) => (
      <Item key={item.to} item={item} onNavigate={props.onNavigate} />
    ))}

    <SectionLabel>Account</SectionLabel>
    {secondary.map((item) => (
      <Item key={item.to} item={item} onNavigate={props.onNavigate} />
    ))}

    <div className="mt-auto pt-4">
      <Item
        item={{
          label: "Settings",
          to: "/settings",
          icon: <IconSettings size={17} />,
        }}
        onNavigate={props.onNavigate}
      />
    </div>
  </nav>
);

export default SideBar;
