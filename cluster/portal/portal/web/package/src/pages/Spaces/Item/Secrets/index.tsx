import { Outlet } from "react-router-dom";

const Workspaces = () => {
  return (
    <>
      <div className="w-full">
        <Outlet />
      </div>
    </>
  );
};

export default Workspaces;
