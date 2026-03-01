import { Outlet } from "react-router-dom";

const UserSecrets = () => {
  return (
    <>
      <div className="w-full">
        <Outlet />
      </div>
    </>
  );
};

export default UserSecrets;
