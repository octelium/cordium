import Footer from "@/components/Footer";
import SideBar from "@/components/SideBar";
import TopBar from "@/components/TopBar";

import { Navigate, Outlet } from "react-router-dom";

import { setStatus } from "@/features/settings/slice";
import { getClientUser, getClientWorkspace } from "@/utils/client";
import { useAppDispatch, useAppSelector } from "@/utils/hooks";

import { useQuery } from "@tanstack/react-query";
import { Toaster } from "react-hot-toast";

import { invalidateSpaces } from "@/utils/octelium";
import { AppShell, Burger } from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import {
  ListSpaceOptions,
  Space,
  Space_Status_Type,
} from "@octelium/apis/main/cordiumv1";

import "@fontsource/ubuntu/400.css";
import "@fontsource/ubuntu/500.css";
import "@fontsource/ubuntu/700.css";

const Root = () => {
  const dispatch = useAppDispatch();
  const [navOpened, { toggle: toggleNav, close: closeNav }] = useDisclosure();
  const consoleWide = useAppSelector(
    (s) => s.settings.terminalWide || s.settings.terminalFullscreen,
  );

  useQuery({
    queryKey: ["user/getStatus"],
    queryFn: async () => {
      const { response } = await getClientUser().getStatus({});
      dispatch(setStatus({ status: response }));

      const wsResp = await getClientWorkspace().listSpace(
        ListSpaceOptions.create({ type: Space_Status_Type.USER }),
      );
      if (wsResp.response && wsResp.response.items.length === 0) {
        await getClientWorkspace().createSpace(
          Space.create({
            metadata: {
              name: `default.${response.user!.metadata!.name}`,
              displayName: "Default Space",
            },
            spec: {},
            status: { type: Space_Status_Type.USER },
          }),
        );
        invalidateSpaces();
      }

      return response;
    },
  });

  return (
    <AppShell
      header={{ height: 60 }}
      navbar={{
        width: 232,
        breakpoint: "md",
        collapsed: { mobile: !navOpened },
      }}
      padding={0}
      className="bg-slate-100"
    >
      <AppShell.Header
        className="border-b border-slate-200 bg-slate-100"
        style={{ backgroundColor: "var(--color-slate-100)" }}
      >
        <div className="flex h-full items-center">
          <Burger
            opened={navOpened}
            onClick={toggleNav}
            hiddenFrom="md"
            size="sm"
            ml="sm"
            aria-label="Toggle navigation"
          />
          <TopBar />
        </div>
      </AppShell.Header>

      <AppShell.Navbar
        className="border-r border-slate-200 bg-slate-100 p-3"
        style={{ backgroundColor: "var(--color-slate-100)" }}
      >
        <SideBar onNavigate={closeNav} />
      </AppShell.Navbar>

      <AppShell.Main className="bg-slate-100">
        <div
          className={
            consoleWide
              ? "w-full px-4 py-6 md:px-8"
              : "mx-auto w-full max-w-[1400px] px-4 py-6 md:px-8"
          }
        >
          <Outlet />
          <Footer />
        </div>
      </AppShell.Main>
    </AppShell>
  );
};

const Page = () => {
  const urlSearchParams = new URLSearchParams(window.location.search);
  const redirect = urlSearchParams.get("redirect");
  if (redirect) {
    return <Navigate to={redirect} replace />;
  }

  return (
    <>
      <Root />
      <Toaster
        position="bottom-center"
        toastOptions={{
          style: {
            borderRadius: "10px",
            fontSize: "0.85rem",
            fontWeight: 500,
          },
        }}
      />
    </>
  );
};

export default Page;
