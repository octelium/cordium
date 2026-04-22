import { Stack, Text, ThemeIcon } from "@mantine/core";
import { IconInbox } from "@tabler/icons-react";
import * as React from "react";

export default (props: { title: string; children?: React.ReactNode }) => {
  return (
    <Stack align="center" justify="center" gap="md" py={64}>
      <ThemeIcon size={48} variant="light" color="gray" radius="xl">
        <IconInbox size={24} />
      </ThemeIcon>
      <Text fw={600} size="lg" c="dimmed" ta="center">
        {props.title}
      </Text>
      {props.children && <div>{props.children}</div>}
    </Stack>
  );
};
