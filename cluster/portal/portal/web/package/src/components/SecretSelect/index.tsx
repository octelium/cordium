import { getClientWorkspace } from "@/utils/client";
import { getShortName } from "@/utils/pb";
import { Select } from "@mantine/core";
import * as WsPB from "@octelium/apis/main/cordiumv1";
import * as MetaPB from "@octelium/apis/main/metav1";
import { useQuery } from "@tanstack/react-query";

const useSpaceSecrets = (spaceRef: MetaPB.ObjectReference) =>
  useQuery({
    queryKey: ["workspace/listSecret", spaceRef.uid, "all"],
    queryFn: () => {
      const { response } = getClientWorkspace().listSecret(
        WsPB.ListSecretOptions.create({
          spaceRef,
          common: { itemsPerPage: 500 },
        }),
      );
      return response;
    },
    enabled: !!spaceRef.uid || !!spaceRef.name,
  });

const SecretSelect = (props: {
  spaceRef: MetaPB.ObjectReference;
  value: string;
  onChange: (value: string) => void;
  label?: string;
  description?: string;
  required?: boolean;
}) => {
  const qry = useSpaceSecrets(props.spaceRef);
  const items = qry.data?.items ?? [];

  return (
    <Select
      label={props.label ?? "Secret"}
      description={
        props.description ??
        "Pick a Secret from this Space. Its value is injected at runtime and never exposed in the spec."
      }
      placeholder={items.length ? "Select a Secret…" : "No Secrets in Space"}
      required={props.required}
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

export default SecretSelect;
