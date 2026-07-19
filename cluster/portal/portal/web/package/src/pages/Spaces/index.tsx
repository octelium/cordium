import * as WsPB from "@octelium/apis/main/cordiumv1";
import { Outlet } from "react-router-dom";

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
