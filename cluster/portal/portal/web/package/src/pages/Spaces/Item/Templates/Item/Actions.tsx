import { DeleteOptions } from "@/apis/metav1/metav1";
import DeleteResource from "@/components/DeleteResource";
import PageWrap from "@/components/PageWrap";
import { useContextSpace } from "@/pages/Spaces/utils";
import { getClientWorkspace } from "@/utils/client";
import { invalidateTemplate } from "@/utils/octelium";
import { useMutation } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";

const Page = () => {
  const ctx = useContextSpace();
  const navigate = useNavigate();
  const client = getClientWorkspace();
  const data = ctx.template.data;
  if (!data || !ctx.space.data) {
    return <></>;
  }
  const mutationDelete = useMutation({
    mutationFn: async () => {
      const { response } = await client.deleteTemplate(
        DeleteOptions.create({ uid: data?.metadata?.uid }),
      );
      return data;
    },
    onSuccess: () => {
      invalidateTemplate(data);
    },
  });

  return (
    <PageWrap qry={ctx.space}>
      {data && (
        <div>
          <div className="flex items-center justify-end">
            <div className="flex flex-1 items-center justify-end">
              <div className="flex flex-none">
                <DeleteResource
                  onDelete={() => {
                    mutationDelete.mutate();
                  }}
                />
              </div>
            </div>
          </div>
        </div>
      )}
    </PageWrap>
  );
};

export default Page;
