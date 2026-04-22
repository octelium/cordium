import { Text } from "@mantine/core";
import * as React from "react";

export const InfoItem = (props: {
  children?: React.ReactNode;
  title: string;
  span?: boolean;
}) => {
  return (
    <div
      style={{
        display: "grid",
        gridTemplateColumns: props.span ? "1fr" : "140px 1fr",
        gap: props.span ? 4 : "0 12px",
        alignItems: "baseline",
        padding: "6px 0",
        borderBottom: "1px solid #f1f5f9",
      }}
    >
      <Text
        size="xs"
        fw={700}
        tt="uppercase"
        style={{
          letterSpacing: "0.06em",
          color: "#94a3b8",
          whiteSpace: "nowrap",
        }}
      >
        {props.title}
      </Text>
      <Text size="sm" fw={700} c="dark.7" style={{ wordBreak: "break-word" }}>
        {props.children}
      </Text>
    </div>
  );
};

export default InfoItem;
