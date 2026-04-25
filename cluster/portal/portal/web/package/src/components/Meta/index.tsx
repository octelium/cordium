import { useEffect } from "react";

const Meta = (props: { title: string }) => {
  useEffect(() => {
    document.title = `${props.title} - Cordium - Octelium`;
  }, [props.title]);

  return null;
};

export default Meta;
