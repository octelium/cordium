import * as React from "react";
import { Link } from "react-router-dom";

const LinkWrap = (props: { to: string; children?: React.ReactNode }) => {
  return (
    <Link
      className="text-cyan-800 hover:text-gray-800 rounded-full transition-all duration-300 shadow-2xl"
      to={props.to}
    >
      {props.children}
    </Link>
  );
};

export default LinkWrap;
