import Footer from "@/components/Footer";
import TopBar from "@/components/TopBar";

import { Navigate, Outlet } from "react-router-dom";

import { setStatus } from "@/features/settings/slice";
import { getClientUser, getClientWorkspace } from "@/utils/client";
import { useAppDispatch } from "@/utils/hooks";

import { useQuery } from "@tanstack/react-query";
import { Toaster } from "react-hot-toast";

import {
  ListSpaceOptions,
  Space,
  Space_Status_Type,
} from "@/apis/cordiumv1/cordiumv1";
import SideBar from "@/components/SideBar";
import { invalidateSpaces } from "@/utils/octelium";
import { AppShell, Burger } from "@mantine/core";
import { useDisclosure, useHeadroom } from "@mantine/hooks";

import "@fontsource/ubuntu/400.css";
import "@fontsource/ubuntu/500.css";
import "@fontsource/ubuntu/700.css";

export default () => {
  const dispatch = useAppDispatch();

  const [opened, { toggle }] = useDisclosure();
  const pinned = useHeadroom({ fixedAt: 120 });

  const urlSearchParams = new URLSearchParams(window.location.search);
  if (urlSearchParams.get("redirect")) {
    const val = urlSearchParams.get("redirect")!;
    urlSearchParams.delete("redirect");
    return <Navigate to={val} />;
  }

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
    <>
      <div className="bg-slate-100 min-h-screen antialiased">
        <AppShell
          header={{ height: 60, collapsed: !pinned, offset: false }}
          navbar={{
            width: 150,
            breakpoint: "sm",
            collapsed: { mobile: !opened },
          }}
          aside={{
            width: 150,
            breakpoint: "md",
            collapsed: { desktop: false, mobile: true },
          }}
          padding="md"
        >
          <AppShell.Header style={{ background: "#f1f5f9" }}>
            <div className="flex flex-row items-center">
              <Burger
                opened={opened}
                onClick={toggle}
                hiddenFrom="sm"
                size="sm"
                ml="sm"
              />
              <TopBar />
            </div>
          </AppShell.Header>

          <AppShell.Navbar
            p="md"
            style={{ background: "#f1f5f9", marginTop: 60 }}
          >
            <SideBar />
          </AppShell.Navbar>

          <AppShell.Main
            style={{
              marginTop: 60,
              background: "#f1f5f9",
              display: "flex",
              flexDirection: "column",
              minHeight: "calc(100vh - 60px)",
            }}
          >
            <div style={{ flex: 1 }}>
              <Outlet />
            </div>
            <Footer />
          </AppShell.Main>

          <AppShell.Aside
            p="md"
            style={{ background: "#f1f5f9", marginTop: 60 }}
          />
        </AppShell>

        <Toaster position="bottom-center" />
      </div>
    </>
  );
};
