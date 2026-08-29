import { getClientWorkspace } from "@/utils/client";
import { getShortName } from "@/utils/pb";
import { Select } from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import { useQuery } from "@tanstack/react-query";

const UserSecretSelect = (props: {
  value: string;
  onChange: (value: string) => void;
  label?: string;
  description?: string;
}) => {
  const qry = useQuery({
    queryKey: ["workspace/listUserSecret", "all"],
    queryFn: () => {
      const { response } = getClientWorkspace().listUserSecret(
        WsPB.ListUserSecretOptions.create({ common: { itemsPerPage: 500 } }),
      );
      return response;
    },
  });

  const items = qry.data?.items ?? [];

  return (
    <Select
      label={props.label ?? "User Secret"}
      description={
        props.description ??
        "One of your personal Secrets. Available in every Workspace you own."
      }
      placeholder={items.length ? "Select a Secret…" : "No User Secrets yet"}
      searchable
      disabled={qry.isPending || items.length === 0}
      data={items.map((x) => ({
        value: x.metadata!.name,
        label: getShortName(x),
      }))}
      value={props.value || null}
      onChange={(val) => val && props.onChange(val)}
    />
  );
};

export default UserSecretSelect;
