import ServiceConnect, {
  CanvasButton,
  CanvasCard,
  CanvasSpinner,
  ConnectHeader,
} from "@/components/ServiceConnect";
import { getErrorMessage, isNotFound } from "@/pages/Agent/utils";
import { getClientUser } from "@/utils/client";
import { hasConnect } from "@/utils/pb";
import { Drawer } from "@mantine/core";
import { useReducedMotion } from "@mantine/hooks";
import { GetOptions } from "@octelium/apis/main/metav1";
import {
  IconAlertCircle,
  IconArrowLeft,
  IconRefresh,
  IconTerminal2,
} from "@tabler/icons-react";
import { useQuery } from "@tanstack/react-query";
import { MotionConfig } from "framer-motion";
import * as React from "react";
import { useLocation, useNavigate, useParams } from "react-router-dom";
import { getServiceTypeLabel } from "../List/utils";

const DRAWER_SIZE = "min(100vw, max(960px, 60vw))";

const Canvas = (props: { title: string; children: React.ReactNode }) => (
  <>
    <ConnectHeader title={props.title} />
    <Drawer.Body className="!flex !min-h-0 !flex-1 !flex-col !p-0">
      <div className="console-surface relative min-h-0 flex-1 bg-console">
        {props.children}
      </div>
    </Drawer.Body>
  </>
);

const Page = () => {
  const { name = "" } = useParams();
  const location = useLocation();
  const navigate = useNavigate();
  const reducedMotion = useReducedMotion();
  const [opened, setOpened] = React.useState(false);

  React.useEffect(() => {
    const frame = requestAnimationFrame(() => setOpened(true));
    return () => cancelAnimationFrame(frame);
  }, []);

  const qry = useQuery({
    queryKey: ["user/getService", name],
    queryFn: async () =>
      (await getClientUser().getService(GetOptions.create({ name }))).response,
    retry: (count, err) => !isNotFound(err) && count < 1,
    refetchOnWindowFocus: false,
  });

  const close = () => setOpened(false);
  const title = `Connect to ${name}`;
  const back = (
    <CanvasButton onClick={close}>
      <IconArrowLeft size={13} />
      Back to Services
    </CanvasButton>
  );

  return (
    <Drawer.Root
      opened={opened}
      onClose={close}
      onExitTransitionEnd={() =>
        navigate(
          { pathname: "/services", search: location.search },
          { replace: true },
        )
      }
      position="right"
      size={DRAWER_SIZE}
      padding={0}
      zIndex={220}
      transitionProps={{ duration: reducedMotion ? 0 : 250 }}
      classNames={{
        content:
          "!flex !flex-col !overflow-hidden border-l border-line !bg-surface",
      }}
    >
      <Drawer.Overlay backgroundOpacity={0.4} blur={2} />
      <Drawer.Content>
        <MotionConfig reducedMotion="user">
          {qry.data && hasConnect(qry.data) ? (
            <ServiceConnect key={qry.data.metadata?.uid} service={qry.data} />
          ) : (
            <Canvas title={title}>
              {qry.isPending ? (
                <CanvasCard
                  title="Looking up the Service"
                  icon={<CanvasSpinner />}
                />
              ) : qry.data ? (
                <CanvasCard
                  title="This Service has no terminal client"
                  icon={<IconTerminal2 size={16} />}
                  subtitle={`${getServiceTypeLabel(qry.data)} Services cannot be reached from a terminal.`}
                  footer={back}
                />
              ) : isNotFound(qry.error) ? (
                <CanvasCard
                  title="This Service is not available"
                  tone="error"
                  icon={<IconAlertCircle size={17} />}
                  subtitle="It does not exist or you do not have access to it."
                  footer={back}
                />
              ) : (
                <CanvasCard
                  title="Could not get the Service"
                  tone="error"
                  icon={<IconAlertCircle size={17} />}
                  subtitle={getErrorMessage(qry.error)}
                  footer={
                    <>
                      {back}
                      <CanvasButton primary onClick={() => void qry.refetch()}>
                        <IconRefresh size={13} />
                        Retry
                      </CanvasButton>
                    </>
                  }
                />
              )}
            </Canvas>
          )}
        </MotionConfig>
      </Drawer.Content>
    </Drawer.Root>
  );
};

export default Page;
