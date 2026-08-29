import { ListResponseMeta } from "@octelium/apis/main/metav1";

import { Pagination, Text } from "@mantine/core";

const Paginator = (props: {
  meta: ListResponseMeta;
  onPageChange: (page: number) => void;
}) => {
  const { meta } = props;
  const perPage = meta.itemsPerPage || 10;
  const totalPages = Math.max(1, Math.ceil(meta.totalCount / perPage));

  if (meta.totalCount <= perPage) {
    return null;
  }

  const from = meta.page * perPage + 1;
  const to = Math.min(meta.totalCount, (meta.page + 1) * perPage);

  return (
    <div className="flex flex-col sm:flex-row items-center justify-between gap-3 pt-2">
      <Text size="xs" c="dimmed" fw={500}>
        {from}–{to} of {meta.totalCount}
      </Text>
      <Pagination
        size="sm"
        radius="md"
        total={totalPages}
        value={meta.page + 1}
        onChange={(v) => props.onPageChange(v - 1)}
      />
    </div>
  );
};

export default Paginator;
