import { useEffect } from "react";

const Meta = (props: { title: string }) => {
  useEffect(() => {
    document.title = `${props.title} · Cordium`;
  }, [props.title]);

  return null;
};

export default Meta;
