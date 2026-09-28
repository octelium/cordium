import { getClientWorkspace } from "@/utils/client";
import { formatMegabytes } from "@/utils";
import { getResourceRef, getShortName, isVolumeShared } from "@/utils/pb";
import { Select } from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import * as MetaPB from "@octelium/apis/main/metav1";
import { useQuery } from "@tanstack/react-query";

const useSpaceVolumes = (spaceRef: MetaPB.ObjectReference) =>
  useQuery({
    queryKey: ["workspace/listVolume", spaceRef.uid, "all"],
    queryFn: () => {
      const { response } = getClientWorkspace().listVolume(
        WsPB.ListVolumeOptions.create({
          spaceRef,
          common: { itemsPerPage: 500 },
        }),
      );
      return response;
    },
    enabled: !!spaceRef.uid || !!spaceRef.name,
  });

const VolumeSelect = (props: {
  spaceRef: MetaPB.ObjectReference;
  value?: MetaPB.ObjectReference;
  onChange: (value: MetaPB.ObjectReference) => void;
  label?: string;
  description?: string;
  required?: boolean;
}) => {
  const qry = useSpaceVolumes(props.spaceRef);
  const items = qry.data?.items ?? [];

  const selected = props.value
    ? items.find(
        (x) =>
          (!!props.value!.uid && x.metadata!.uid === props.value!.uid) ||
          (!!props.value!.name && x.metadata!.name === props.value!.name),
      )
    : undefined;

  return (
    <Select
      label={props.label ?? "Volume"}
      description={
        props.description ?? "Pick a Volume from this Space to mount."
      }
      placeholder={items.length ? "Select a Volume…" : "No Volumes in Space"}
      required={props.required}
      searchable
      disabled={qry.isPending || items.length === 0}
      data={items.map((x) => ({
        value: x.metadata!.uid,
        label: `${getShortName(x)} · ${formatMegabytes(
          x.spec?.size?.megabytes ?? 0,
        )}${isVolumeShared(x) ? " · shared" : ""}`,
      }))}
      value={selected?.metadata?.uid ?? null}
      onChange={(val) => {
        const found = items.find((x) => x.metadata!.uid === val);
        if (found) props.onChange(getResourceRef(found));
      }}
    />
  );
};

export default VolumeSelect;
