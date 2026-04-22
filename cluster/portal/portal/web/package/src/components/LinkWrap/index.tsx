import { Anchor } from "@mantine/core";
import * as React from "react";
import { Link } from "react-router-dom";

const LinkWrap = (props: { to: string; children?: React.ReactNode }) => {
  return (
    <Anchor
      component={Link}
      to={props.to}
      size="sm"
      fw={700}
      underline="hover"
      className="transition-all duration-500"
    >
      {props.children}
    </Anchor>
  );
};

export default LinkWrap;
