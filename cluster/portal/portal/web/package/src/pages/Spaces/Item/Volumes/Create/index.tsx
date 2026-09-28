import Meta from "@/components/Meta";
import MetadataEdit from "@/components/MetadataEdit";
import PageHeader from "@/components/PageHeader";
import Panel, { PanelBody, PanelFooter, PanelHeader } from "@/components/Panel";
import RegionSelect from "@/components/RegionSelect";
import { useContextSpace } from "@/pages/Spaces/utils";
import { onError } from "@/utils";
import { getClientWorkspace } from "@/utils/client";
import { getPathSpace, invalidateVolumes } from "@/utils/octelium";
import { getShortName } from "@/utils/pb";
import {
  Button,
  NumberInput,
  SegmentedControl,
  Stack,
  Text,
} from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import * as MetaPB from "@octelium/apis/main/metav1";
import { IconDatabase, IconServer } from "@tabler/icons-react";
import { useMutation } from "@tanstack/react-query";
import * as React from "react";
import toast from "react-hot-toast";
import { useNavigate } from "react-router-dom";

const CreateVolume = () => {
  const ctx = useContextSpace();
  const client = getClientWorkspace();
  const navigate = useNavigate();
  const space = ctx.space.data;

  const [req, setReq] = React.useState(
    WsPB.Volume.create({
      apiVersion: "cordium/v1",
      kind: "Volume",
      metadata: {},
      spec: {
        size: { megabytes: 10000 },
        accessMode: WsPB.Volume_AccessMode.EXCLUSIVE,
      },
      status: {},
    }),
  );
  const [region, setRegion] = React.useState("");

  const mutation = useMutation({
    mutationFn: async () => {
      const payload = WsPB.Volume.clone(req);
      if (region) {
        payload.status!.regionRef = MetaPB.ObjectReference.create({
          name: region,
        });
      }
      const { response } = await client.createVolume(payload);
      return response;
    },
    onSuccess: () => {
      invalidateVolumes();
      toast.success("Volume created");
      navigate(`${getPathSpace(space!)}/volumes`);
    },
    onError,
  });

  if (!space) return null;

  const shared = req.spec?.accessMode === WsPB.Volume_AccessMode.SHARED;

  return (
    <>
      <Meta title="New Volume" />
      <PageHeader
        title="New Volume"
        crumbs={[
          { label: "Spaces", to: "/spaces" },
          { label: getShortName(space), to: getPathSpace(space) },
          { label: "Volumes", to: `${getPathSpace(space)}/volumes` },
          { label: "New" },
        ]}
        description={`Persistent storage that Templates and Workspaces in ${getShortName(space)} can mount.`}
      />

      <div className="max-w-3xl">
        <Stack gap="lg">
          <Panel>
            <PanelHeader
              icon={<IconDatabase size={16} />}
              title="Identity"
              description="Referenced by this name from the volume mounts of a spec."
            />
            <PanelBody>
              <MetadataEdit
                metadata={req.metadata!}
                parentName={space.metadata?.name}
                onChange={(md) => {
                  const next = WsPB.Volume.clone(req);
                  next.metadata = md;
                  setReq(next);
                }}
              />
            </PanelBody>
          </Panel>

          <Panel>
            <PanelHeader
              icon={<IconServer size={16} />}
              title="Storage"
              description="The size can be grown later but never shrunk. The access mode is immutable."
            />
            <PanelBody>
              <Stack gap="md">
                <NumberInput
                  label="Size"
                  description="Megabytes of storage. 10000 = 10 GB."
                  placeholder="10000"
                  min={1}
                  max={10000000}
                  step={1000}
                  value={req.spec?.size?.megabytes ?? 0}
                  onChange={(v) => {
                    const n = typeof v === "number" ? v : Number(v) || 0;
                    const next = WsPB.Volume.clone(req);
                    next.spec!.size = WsPB.Volume_Spec_Size.create({
                      megabytes: n,
                    });
                    setReq(next);
                  }}
                />

                <div>
                  <Text size="sm" fw={500} mb={2}>
                    Access mode
                  </Text>
                  <Text size="xs" c="dimmed" mb={8}>
                    {shared
                      ? "Several Workspaces can mount it at the same time. Requires a shared filesystem storage backend."
                      : "Actively mounted by a single Workspace at a time."}
                  </Text>
                  <SegmentedControl
                    size="xs"
                    value={shared ? "shared" : "exclusive"}
                    onChange={(v) => {
                      const next = WsPB.Volume.clone(req);
                      next.spec!.accessMode =
                        v === "shared"
                          ? WsPB.Volume_AccessMode.SHARED
                          : WsPB.Volume_AccessMode.EXCLUSIVE;
                      setReq(next);
                    }}
                    data={[
                      { label: "Exclusive", value: "exclusive" },
                      { label: "Shared", value: "shared" },
                    ]}
                  />
                </div>

                <RegionSelect
                  description="Only Workspaces running in this region can mount the Volume."
                  value={region}
                  onChange={setRegion}
                />
              </Stack>
            </PanelBody>
            <PanelFooter>
              <Button variant="default" onClick={() => navigate(-1)}>
                Cancel
              </Button>
              <Button
                loading={mutation.isPending}
                disabled={!req.metadata?.name}
                onClick={() => mutation.mutate()}
              >
                Create Volume
              </Button>
            </PanelFooter>
          </Panel>
        </Stack>
      </div>
    </>
  );
};

export default CreateVolume;
