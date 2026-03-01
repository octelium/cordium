import { GetOptions } from "@/apis/metav1/metav1";
import { getClientWorkspace } from "@/utils/client";
import { useQuery } from "@tanstack/react-query";
import * as React from "react";
import { match } from "ts-pattern";

const Block = (props: { children?: React.ReactNode }) => {
  return (
    <div className="bg-gray-200 font-bold text-black">{props.children}</div>
  );
};

const ParentTemplate = (props: { uid: string }) => {
  const { uid } = props;
  const qry = useQuery({
    queryKey: ["workspace/getTemplate", uid],
    queryFn: () => {
      const { response } = getClientWorkspace().getTemplate(
        GetOptions.create({ uid }),
      );
      return response;
    },
  });

  if (!qry.isSuccess) {
    return <></>;
  }

  return <Block>Template: {qry.data.metadata?.name}</Block>;
};

const ParentSpace = (props: { uid: string }) => {
  const { uid } = props;
  const qry = useQuery({
    queryKey: ["workspace/getSpace", uid],
    queryFn: () => {
      const { response } = getClientWorkspace().getSpace(
        GetOptions.create({ uid }),
      );
      return response;
    },
  });

  if (!qry.isSuccess) {
    return <></>;
  }

  return <Block>Space: {qry.data.metadata?.name}</Block>;
};

const Parent = (props: { uid: string; kind: "Template" | "Space" }) => {
  const { uid } = props;
  return match(props.kind)
    .with("Space", () => <ParentSpace uid={uid} />)
    .otherwise(() => <></>);
};

export default Parent;
