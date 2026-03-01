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

export default () => {
  const dispatch = useAppDispatch();

  const [opened, { toggle }] = useDisclosure();
  const pinned = useHeadroom({ fixedAt: 120 });

  const urlSearchParams = new URLSearchParams(window.location.search);
  if (urlSearchParams.get("redirect")) {
    const val = urlSearchParams.get("redirect")!;
    urlSearchParams.delete("redirect");
    console.log("Redirecting to", val);
    return <Navigate to={val} />;
  }

  useQuery({
    queryKey: ["user/getStatus"],
    queryFn: async () => {
      const { response } = await getClientUser().getStatus({});
      console.log("getStatus", response);
      dispatch(setStatus({ status: response }));

      let wsResp = await getClientWorkspace().listSpace(
        ListSpaceOptions.create({
          type: Space_Status_Type.USER,
        }),
      );
      if (wsResp.response && wsResp.response.items.length === 0) {
        await getClientWorkspace().createSpace(
          Space.create({
            metadata: {
              name: `default.${response.user!.metadata!.name}`,
              displayName: "Default Space",
            },
            spec: {},
            status: {
              type: Space_Status_Type.USER,
            },
          }),
        );

        invalidateSpaces();
      }

      return response;
    },
  });

  return (
    <>
      <div>
        <title>Cordium - Octelium Homepage</title>

        <div className="bg-slate-100! min-h-screen antialiased">
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
            <AppShell.Header className="bg-slate-100!">
              <div className="flex flex-row items-center justify-center">
                <Burger
                  opened={opened}
                  onClick={toggle}
                  hiddenFrom="sm"
                  size="sm"
                />
                <TopBar />
              </div>
            </AppShell.Header>

            <AppShell.Navbar p="md" className="bg-slate-100! mt-[60px]">
              <SideBar />
            </AppShell.Navbar>

            <AppShell.Main className="h-full w-full mt-[60px]">
              <div className="flex-1 flex flex-col min-h-full min-w-full items-center justify-center">
                <div className="flex-1 w-full h-full mt-[60px]">
                  <Outlet />
                </div>
              </div>
            </AppShell.Main>
            <AppShell.Aside
              p="md"
              className="bg-slate-100! mt-[60px]"
            ></AppShell.Aside>
          </AppShell>
          {/*
          <TopBar />
          <div className="mb-8"></div>
         */}

          {/*
          
          <div className="flex-1 w-full flex flex-col items-center">
            <div className="md:container mx-auto mt-2 p-2 md:p-8 w-full !max-w-4xl">
              <div className="w-full mb-8">
                <MainBar />
              </div>
              <div className="w-full mb-8">
                <TitleItem />
              </div>
              <div>
                <Outlet />
              </div>
            </div>
          </div>
          */}

          <Toaster position="bottom-center" />
          <Footer />
        </div>
      </div>
    </>
  );
};
