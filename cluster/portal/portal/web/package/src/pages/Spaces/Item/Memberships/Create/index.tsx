import * as WsPB from "@/apis/cordiumv1/cordiumv1";
import * as React from "react";

import { getClientWorkspace } from "@/utils/client";

import { GetOptions, ObjectReference } from "@/apis/metav1/metav1";
import { onError } from "@/utils";
import { getResourceRef } from "@/utils/pb";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import toast from "react-hot-toast";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";

import Field from "@/components/Field";
import ItemContainer from "@/components/ItemContainer";

import { Button, Select } from "@mantine/core";

const CreateEnvironment = () => {
  let [searchParams, _] = useSearchParams();

  let { spaceUID } = useParams();

  if (!spaceUID) {
    return <></>;
  }

  const spaceQuery = useQuery({
    queryKey: ["workspace/getSpace", spaceUID],
    queryFn: () => {
      const { response } = getClientWorkspace().getSpace(
        GetOptions.create({ uid: spaceUID }),
      );
      return response;
    },
  });

  let [req, setReq] = React.useState(
    WsPB.CreateMembershipRequest.create({
      spaceRef: getResourceRef(spaceQuery.data!),
      userType: {
        oneofKind: "email",
        email: "",
      },
    }),
  );

  const client = getClientWorkspace();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const mutation = useMutation({
    mutationFn: async () => {
      req.spaceRef = getResourceRef(spaceQuery.data!);
      const { response } = await client.createMembership(req);

      return response;
    },
    onSuccess: (data) => {
      queryClient.refetchQueries({
        queryKey: ["workspace/getSpace", spaceQuery.data?.metadata?.uid],
      });
      navigate(`/spaces/uid/${spaceQuery.data?.metadata?.uid}`);
      toast.success(`Membership created`);
    },
    onError: onError,
  });

  if (!spaceQuery.isSuccess) {
    return <></>;
  }

  return (
    <div>
      <div className="w-full mb-4">
        <ItemContainer title="Type" isHorizontal>
          <Select
            data={[
              {
                label: "Email",
                value: "email",
              },
              {
                label: "User name",
                value: "user",
              },
            ]}
            defaultValue={"user"}
            onChange={(val) => {
              switch (val) {
                case "email":
                  req.userType = {
                    oneofKind: "email",
                    email: "",
                  };
                  break;
                case "user":
                  req.userType = {
                    oneofKind: "userRef",
                    userRef: ObjectReference.create(),
                  };
                  break;
              }
              setReq(WsPB.CreateMembershipRequest.clone(req));
            }}
          />
        </ItemContainer>

        {req.userType.oneofKind === `email` && (
          <Field
            label="Email"
            val={req.userType.email}
            onChange={(val) => {
              let f = req.userType as {
                oneofKind: `email`;
                email: string;
              };
              f.email = val as string;
              setReq(WsPB.CreateMembershipRequest.clone(req));
            }}
          />
        )}

        {req.userType.oneofKind === `userRef` && (
          <Field
            label="User name"
            val={req.userType.userRef.name}
            onChange={(val) => {
              let f = req.userType as {
                oneofKind: `userRef`;
                userRef: ObjectReference;
              };
              f.userRef.name = val as string;
              setReq(WsPB.CreateMembershipRequest.clone(req));
            }}
          />
        )}
      </div>
      <div>
        <ItemContainer title="Role" isHorizontal>
          <Select
            data={[
              {
                label: "Ordinary User",
                value:
                  WsPB.CreateMembershipRequest_Role[
                    WsPB.CreateMembershipRequest_Role.USER
                  ],
              },
              {
                label: "Administrator",
                value:
                  WsPB.CreateMembershipRequest_Role[
                    WsPB.CreateMembershipRequest_Role.ADMIN
                  ],
              },
              {
                label: "Owner",
                value:
                  WsPB.CreateMembershipRequest_Role[
                    WsPB.CreateMembershipRequest_Role.OWNER
                  ],
              },
            ]}
            defaultValue={
              WsPB.CreateMembershipRequest_Role[
                WsPB.CreateMembershipRequest_Role.USER
              ]
            }
            onChange={(val) => {
              req.role = WsPB.CreateMembershipRequest_Role[val as "USER"];
              setReq(WsPB.CreateMembershipRequest.clone(req));
            }}
          />
        </ItemContainer>
      </div>

      <div className="flex items-center justify-end">
        <Button
          variant="outline"
          onClick={() => {
            navigate(-1);
          }}
        >
          Cancel
        </Button>
        <Button
          onClick={() => {
            mutation.mutate();
          }}
        >
          Create
        </Button>
      </div>
    </div>
  );
};

export default CreateEnvironment;
