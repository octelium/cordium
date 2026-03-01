import * as React from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { twMerge } from "tailwind-merge";

import { Button } from "@mantine/core";
import {
  MdOutlineKeyboardArrowLeft,
  MdOutlineKeyboardArrowRight,
} from "react-icons/md";

const Item = (props: {
  children?: React.ReactNode;
  title: string;
  path: string;
  active?: boolean;
}) => {
  const navigate = useNavigate();
  return (
    <button
      className={twMerge(
        "w-full flex-1 flex items-center justify-center py-2 px-4",
        "font-bold text-sm",
        "transitional-all duration-300",
        " hover:bg-white hover:bg-opacity-75 rounded-lg",
        "hover:shadow-sm border-b-4 rounded-b-none border-transparent",
        "mx-2 my-2",
        props.active
          ? `text-black border-black`
          : `text-slate-600 hover:text-slate-800`,
      )}
      onClick={() => {
        navigate(props.path);
      }}
    >
      {props.title}
    </button>
  );
};
const MainBar = () => {
  const loc = useLocation();
  const navigate = useNavigate();

  return (
    <div className="w-full flex items-center justify-between">
      <div className="flex flex-col md:flex-row flex-1 w-full flex-nowrap items-center justify-center">
        <Item
          title="Workspaces"
          path="/workspaces"
          active={loc.pathname === "/workspaces"}
        />
        <Item
          title="Spaces"
          path="/spaces"
          active={loc.pathname === "/spaces"}
        />
        <Item
          title="Services"
          path="/services"
          active={loc.pathname === "/services"}
        />

        <Item
          title="Your Secrets"
          path="/usersecrets"
          active={loc.pathname === "/usersecrets"}
        />
      </div>
      <div className="hidden md:flex">
        <Button
          variant="none"
          onClick={() => {
            navigate(-1);
          }}
        >
          <span className="font-extrabold text-black">
            <MdOutlineKeyboardArrowLeft />
          </span>
        </Button>
        <Button
          variant="none"
          onClick={() => {
            navigate(1);
          }}
        >
          <span className="font-extrabold text-black">
            <MdOutlineKeyboardArrowRight />
          </span>
        </Button>
      </div>
    </div>
  );
};

export default MainBar;
