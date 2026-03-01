import PageTitle from "@/components/PageTitle";
import { Outlet } from "react-router-dom";

const Workspaces = () => {
  return (
    <>
      <PageTitle title="Workspaces" />
      <div className="w-full">
        <Outlet />
      </div>
    </>
  );
};

export default Workspaces;
