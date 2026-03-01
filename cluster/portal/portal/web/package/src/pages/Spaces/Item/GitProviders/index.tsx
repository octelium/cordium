import { Outlet } from "react-router-dom";

const GitProviders = () => {
  return (
    <>
      <div className="w-full">
        <Outlet />
      </div>
    </>
  );
};

export default GitProviders;
