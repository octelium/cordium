import { ListResponseMeta } from "@/apis/metav1/metav1";
import { useNavigate } from "react-router-dom";
import { twMerge } from "tailwind-merge";

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

  return (
    <div className="w-full flex items-center justify-center">
      <div className="w-full flex items-center justify-center flex-wrap">
        {[...Array(totalPages)].map((e, i) => {
          return (
            <button
              key={i}
              className={twMerge(
                `flex items-center text-center justify-center`,
                "mx-2 my-2  text-white font-bold py-1 px-2 rounded-md shadow-2xl",
                meta.page === i
                  ? `bg-slate-900 border-[1px] border-slate-900`
                  : `bg-transparent border-[1px] border-slate-900 text-slate-700`,
              )}
              onClick={() => {
                if (props.onPageChange) {
                  props.onPageChange(i);
                } else if (props.path) {
                  navigate(
                    `${props.path}${
                      props.path.includes("?") ? "&" : "?"
                    }page=${i}`,
                  );
                }
              }}
            >
              {i + 1}
            </button>
          );
        })}
      </div>
    </div>
  );
};
export default Paginator;
