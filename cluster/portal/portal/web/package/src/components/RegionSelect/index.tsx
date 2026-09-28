import { getClientWorkspace } from "@/utils/client";
import { Select } from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import { useQuery } from "@tanstack/react-query";

const RegionSelect = (props: {
  value: string;
  onChange: (val: string) => void;
  label?: string;
  description?: string;
}) => {
  const qry = useQuery({
    queryKey: ["workspace/listRegion"],
    queryFn: () => {
      const { response } = getClientWorkspace().listRegion(
        WsPB.ListRegionOptions.create({}),
      );
      return response;
    },
  });

  const items = qry.data?.items ?? [];
  if (items.length < 2) return null;

  return (
    <Select
      label={props.label ?? "Region"}
      description={
        props.description ??
        "Where the Workspace runs. Defaults to your preferred region."
      }
      placeholder="Default"
      clearable
      data={items.map((x) => ({
        value: x.metadata!.name,
        label: [x.metadata!.name, x.status?.city, x.status?.country]
          .filter(Boolean)
          .join(" · "),
      }))}
      value={props.value || null}
      onChange={(val) => props.onChange(val ?? "")}
    />
  );
};

export default RegionSelect;
