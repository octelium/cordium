/// <reference types="vite-plugin-svgr/client" />

import { useAppSelector } from "@/utils/hooks";
import { useNavigate } from "react-router-dom";

const TopBar = () => {
  const navigate = useNavigate();
  const settings = useAppSelector((state) => state.settings);
  const picURL =
    settings.status?.session?.metadata?.picURL ??
    settings.status?.user?.metadata?.picURL;

  return (
    <nav className="w-full h-[60px] border-b-0 border-slate-300 flex px-4">
      <button
        className="flex-none flex items-center justify-center cursor-pointer"
        onClick={() => {
          navigate("/");
        }}
      ></button>
      <div className="grow"></div>
      <div className="flex-none flex items-center">
        <div className="flex items-center justify-center align-middle">
          <div className="w-10 h-10 rounded-full border-white border-2 text-gray-600 hover:text-gray-900 font-bold transition-all duration-300">
            {picURL ? (
              <img
                className="rounded-full w-full h-full"
                src={picURL}
                alt="User pic"
              />
            ) : (
              <div className="rounded-full bg-sky-600 hover:bg-indigo-800 transition-all duration-300 w-full h-full"></div>
            )}
          </div>
        </div>
      </div>
    </nav>
  );
};

export default TopBar;
