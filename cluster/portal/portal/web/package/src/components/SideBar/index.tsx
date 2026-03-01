import { Link, useLocation } from "react-router-dom";
import { twMerge } from "tailwind-merge";

import { FaLayerGroup } from "react-icons/fa6";

import { FaTerminal } from "react-icons/fa";
import { MdOutlineSettings } from "react-icons/md";

const items = [
  {
    title: "Spaces",
    url: "/spaces",
    icon: FaLayerGroup,
  },
  {
    title: "Workspaces",
    url: "/workspaces",
    icon: FaTerminal,
  },
];

export default function () {
  const loc = useLocation();

  return (
    <div className="min-h-full w-full">
      <div className="flex flex-col h-full w-full">
        <div>
          {items.map((item) => (
            <div key={item.title}>
              <div>
                <Link
                  className={twMerge(
                    "transition-all duration-500 hover:bg-slate-200 font-extrabold",
                    "flex w-full items-center justify-center",
                    "py-2 px-2 rounded-md my-1",
                    "text-sm",
                    loc.pathname.startsWith(item.url)
                      ? `!text-white bg-zinc-800 hover:bg-black shadow`
                      : `text-zinc-600 hover:text-zinc-800`,
                  )}
                  to={item.url}
                >
                  <item.icon />
                  <span className="flex-1 ml-2">{item.title}</span>
                </Link>
              </div>
            </div>
          ))}
        </div>
        <div className="flex">
          <Link
            className={twMerge(
              "transition-all duration-500 hover:bg-slate-200 font-extrabold",
              "flex w-full items-center justify-center",
              "py-2 px-2 rounded-md my-1",
              "text-sm",
              loc.pathname.startsWith(`/settings`)
                ? `!text-white bg-zinc-800 hover:bg-black`
                : `text-zinc-600 hover:text-zinc-800`,
            )}
            to={`/settings`}
          >
            <MdOutlineSettings />
            <span className="flex-1 ml-2">Settings</span>
          </Link>
        </div>
      </div>
    </div>
  );
}
