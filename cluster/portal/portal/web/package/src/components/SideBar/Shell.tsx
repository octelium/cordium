import { AppShell, Drawer } from "@mantine/core";
import {
  useLocalStorage,
  useMediaQuery,
  useReducedMotion,
} from "@mantine/hooks";
import { Menu } from "lucide-react";
import * as React from "react";
import { useLocation } from "react-router-dom";
import SideBar from ".";

const PortalShell = (props: {
  header: React.ReactNode;
  children: React.ReactNode;
}) => {
  const location = useLocation();
  const desktop = useMediaQuery("(min-width: 48em)");
  const reducedMotion = useReducedMotion();
  const [collapsed, setCollapsed] = useLocalStorage({
    key: "cordium-portal-sidebar-collapsed",
    defaultValue: false,
    getInitialValueInEffect: false,
    deserialize: (value) => value === "true",
  });
  const [drawerRoute, setDrawerRoute] = React.useState<string | null>(null);
  const drawerOpened = !desktop && drawerRoute === location.key;
  const closeDrawer = () => setDrawerRoute(null);

  if (desktop && drawerRoute !== null) setDrawerRoute(null);

  return (
    <AppShell
      header={{ height: 60 }}
      navbar={{
        width: collapsed ? 64 : 248,
        breakpoint: "sm",
        collapsed: { mobile: true },
      }}
      padding={0}
      transitionDuration={reducedMotion ? 0 : 240}
      transitionTimingFunction="cubic-bezier(0.22, 1, 0.36, 1)"
      className="bg-canvas"
    >
      <AppShell.Header
        className="!border-line !bg-canvas"
        style={{ zIndex: 200 }}
      >
        <div className="flex h-full items-center">
          <button
            type="button"
            aria-label="Open navigation"
            aria-haspopup="dialog"
            aria-expanded={drawerOpened}
            onClick={() => setDrawerRoute(location.key)}
            className="ml-2 flex h-11 w-11 shrink-0 items-center justify-center rounded-lg text-ink-soft outline-none hover:bg-surface-strong/60 focus-visible:ring-2 focus-visible:ring-focus min-[48em]:hidden"
          >
            <Menu size={20} aria-hidden />
          </button>
          {props.header}
        </div>
      </AppShell.Header>

      <AppShell.Navbar
        p={0}
        className="!border-line !bg-canvas"
        inert={!desktop}
        aria-label="Portal sidebar"
      >
        <SideBar
          collapsed={collapsed}
          onToggle={() => setCollapsed(!collapsed)}
        />
      </AppShell.Navbar>

      <AppShell.Main className="bg-canvas">{props.children}</AppShell.Main>

      <Drawer.Root
        opened={drawerOpened}
        onClose={closeDrawer}
        position="left"
        size="min(320px, calc(100vw - 24px))"
        padding={0}
        zIndex={220}
        transitionProps={{ duration: reducedMotion ? 0 : 240 }}
      >
        <Drawer.Overlay backgroundOpacity={0.28} />
        <Drawer.Content
          aria-label="Portal navigation"
          className="!overflow-hidden !bg-canvas"
        >
          <Drawer.Body className="!h-full !p-0">
            <SideBar onClose={closeDrawer} />
          </Drawer.Body>
        </Drawer.Content>
      </Drawer.Root>
    </AppShell>
  );
};

export default PortalShell;
