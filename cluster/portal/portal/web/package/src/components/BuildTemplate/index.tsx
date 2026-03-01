import * as WsPB from "@/apis/cordiumv1/cordiumv1";
import { onError } from "@/utils";
import { getClientWorkspace } from "@/utils/client";
import { getResourceRef } from "@/utils/pb";
import { useMutation } from "@tanstack/react-query";
import * as React from "react";
import { toast } from "react-hot-toast";

import { invalidateResource } from "@/utils/octelium";

import { Button, Modal, TagsInput } from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";

const BuildTemplate = (props: { item: WsPB.Template }) => {
  const client = getClientWorkspace();
  const { item } = props;

  let [tags, setTags] = React.useState(["latest"]);

  const [opened, { open, close }] = useDisclosure(false);

  let [buildTemplateReq, setBuildTemplateReq] = React.useState(
    WsPB.BuildTemplateRequest.create({
      templateRef: getResourceRef(item!),
    }),
  );

  const mutationBuild = useMutation({
    mutationFn: async () => {
      const { response } = await client.buildTemplate(buildTemplateReq);

      return { response };
    },
    onSuccess: ({ response }) => {
      setTags(["latest"]);
      invalidateResource(item);
      toast.success(`Initialized a new Build`);
    },
    onError,
  });

  return (
    <>
      <Button size="lg" onClick={open}>
        Build Template
      </Button>

      <Modal opened={opened} onClose={close} centered>
        <div className="font-bold text-xl mb-4">
          <div className="w-full py-4 px-2">
            {item.status?.buildInfo?.builds && (
              <div>
                <div className="mb-4 text-lg text-zinc-700 font-bold">
                  Choose one or more tags for your Build
                </div>
                <div>
                  <TagsInput
                    multiple
                    value={tags}
                    onChange={(v) => {
                      setTags(v);
                      buildTemplateReq.tags = v;
                      setBuildTemplateReq(
                        WsPB.BuildTemplateRequest.clone(buildTemplateReq),
                      );
                    }}
                  />
                </div>
              </div>
            )}
          </div>
        </div>

        <div className="mt-4 flex justify-end items-center">
          <Button variant="outline" onClick={close}>
            Cancel
          </Button>
          <Button
            className="ml-4"
            loading={mutationBuild.isPending}
            onClick={() => {
              mutationBuild.mutate();
            }}
            autoFocus
          >
            Build
          </Button>
        </div>
      </Modal>
    </>
  );
};

export default BuildTemplate;
