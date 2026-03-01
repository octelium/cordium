import * as React from "react";
import { Outlet, useOutletContext } from "react-router-dom";
import * as WsPB from "@/apis/cordiumv1/cordiumv1";

const Page = () => {
  return (
    <>
      <Outlet />
    </>
  );
};

export default Page;

export interface ContextSpace {
  space?: WsPB.Space;
  template?: WsPB.Template;
}
