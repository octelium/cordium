import { Tooltip } from "@mantine/core";
import {
  Camera,
  KeyRound,
  Layers,
  LayoutDashboard,
  PanelLeftClose,
  PanelLeftOpen,
  Server,
  Settings,
  Sparkles,
  SquareTerminal,
  X,
  type LucideIcon,
} from "lucide-react";
import { NavLink, useLocation } from "react-router-dom";
import { twMerge } from "tailwind-merge";

interface NavItem {
  label: string;
  to: string;
  icon: LucideIcon;
  end?: boolean;
}

const primary: NavItem[] = [
  { label: "Overview", to: "/", icon: LayoutDashboard, end: true },
  { label: "Agent", to: "/agent", icon: Sparkles },
];

const resources: NavItem[] = [
  { label: "Spaces", to: "/spaces", icon: Layers },
  { label: "Workspaces", to: "/workspaces", icon: SquareTerminal },
  { label: "Snapshots", to: "/snapshots", icon: Camera },
];

const access: NavItem[] = [
  { label: "Services", to: "/services", icon: Server },
  { label: "Your Secrets", to: "/usersecrets", icon: KeyRound },
];

const Item = (props: {
  item: NavItem;
  collapsed: boolean;
  onNavigate?: () => void;
}) => {
  const { pathname } = useLocation();
  const selected =
    pathname === props.item.to ||
    (!props.item.end && pathname.startsWith(`${props.item.to}/`));
  return (
    <Tooltip
      label={props.item.label}
      disabled={!props.collapsed}
      position="right"
      events={{ hover: props.collapsed, focus: props.collapsed, touch: false }}
    >
      <NavLink
        to={props.item.to}
        end={props.item.end}
        aria-label={props.item.label}
        onClick={props.onNavigate}
        className={twMerge(
          "flex h-9 shrink-0 items-center gap-2.5 rounded-lg border px-2.5 text-[0.82rem] outline-none transition-colors duration-150 focus-visible:ring-2 focus-visible:ring-focus [@media(pointer:coarse)]:h-11",
          props.collapsed && "justify-center px-0",
          selected
            ? "border-line bg-surface font-semibold text-ink shadow-panel"
            : "border-transparent font-normal text-ink-soft hover:bg-surface-strong/50 hover:text-ink",
        )}
      >
        <props.item.icon
          size={16}
          strokeWidth={1.8}
          className="shrink-0"
          aria-hidden
        />
        {!props.collapsed && (
          <span className="truncate">{props.item.label}</span>
        )}
      </NavLink>
    </Tooltip>
  );
};

const SideBar = (props: {
  collapsed?: boolean;
  onToggle?: () => void;
  onClose?: () => void;
  onNavigate?: () => void;
}) => {
  const collapsed = props.collapsed ?? false;
  const toggleLabel = props.onClose
    ? "Close navigation"
    : collapsed
      ? "Expand sidebar"
      : "Collapse sidebar";
  const ToggleIcon = props.onClose
    ? X
    : collapsed
      ? PanelLeftOpen
      : PanelLeftClose;
  const renderItem = (item: NavItem) => (
    <Item
      key={item.to}
      item={item}
      collapsed={collapsed}
      onNavigate={props.onNavigate ?? props.onClose}
    />
  );

  return (
    <div className="flex h-full min-h-0 flex-col bg-canvas">
      <div
        className={twMerge(
          "flex h-12 shrink-0 items-center justify-between px-3",
          collapsed && "justify-center px-0",
        )}
      >
        {!collapsed && (
          <span className="text-xs font-semibold text-ink-muted">Portal</span>
        )}
        {(props.onToggle || props.onClose) && (
          <Tooltip label={toggleLabel} position="right">
            <button
              type="button"
              aria-label={toggleLabel}
              aria-expanded={props.onClose ? undefined : !collapsed}
              onClick={props.onClose ?? props.onToggle}
              className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-ink-muted outline-none transition-colors hover:bg-surface-strong/60 hover:text-ink focus-visible:ring-2 focus-visible:ring-focus [@media(pointer:coarse)]:h-11 [@media(pointer:coarse)]:w-11"
            >
              <ToggleIcon size={17} aria-hidden />
            </button>
          </Tooltip>
        )}
      </div>

      <nav
        aria-label="Portal"
        className="min-h-0 flex-1 overflow-y-auto overscroll-contain [scrollbar-width:thin]"
      >
        <div className="space-y-1 px-2 pb-3">{primary.map(renderItem)}</div>
        {[
          { label: "Resources", items: resources },
          { label: "Access", items: access },
        ].map((group) => (
          <div
            key={group.label}
            className="space-y-1 border-t border-line/80 px-2 py-3"
          >
            {!collapsed && (
              <p className="px-2.5 pb-1 text-xs font-semibold text-ink-muted">
                {group.label}
              </p>
            )}
            {group.items.map(renderItem)}
          </div>
        ))}
      </nav>

      <div className="shrink-0 border-t border-line/80 p-2">
        {renderItem({ label: "Settings", to: "/settings", icon: Settings })}
      </div>
    </div>
  );
};

export default SideBar;
