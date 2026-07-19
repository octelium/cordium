import { ListResponseMeta } from "@octelium/apis/main/metav1";
import { useNavigate } from "react-router-dom";

import { Pagination } from "@mantine/core";

const Paginator = (props: {
  meta: ListResponseMeta;
  path?: string;
  onPageChange?: (page: number) => void;
}) => {
  const { meta } = props;
  const navigate = useNavigate();
  const totalPages = Math.ceil(meta.totalCount / meta.itemsPerPage);

  if (meta.page == 0 && meta.totalCount <= meta.itemsPerPage) {
    return <></>;
  }

  return (
    <div className="flex items-center justify-center">
      <Pagination
        total={totalPages}
        radius={"xl"}
        value={meta.page + 1}
        onChange={(v) => {
          const i = v - 1;
          if (props.onPageChange) {
            props.onPageChange(i);
          } else if (props.path) {
            navigate(
              `${props.path}${props.path.includes("?") ? "&" : "?"}page=${i}`,
            );
          }
        }}
      />
    </div>
  );
};
export default Paginator;
