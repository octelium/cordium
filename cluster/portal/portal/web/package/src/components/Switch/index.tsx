import { Switch as SwitchI } from "@mantine/core";

const Switch = (props: {
  val: boolean | undefined;
  onChange: (v: boolean) => void;
  label?: string;
  description?: string;
}) => {
  return (
    <SwitchI
      label={props.label}
      checked={props.val}
      description={props.description}
      onChange={(event) => {
        props.onChange(event.currentTarget.checked);
      }}
    />
  );
  /*
  return (
    <SwitchI
      checked={props.val ?? false}
      onChange={(e) => {
        props.onChange(e);
      }}
      className={`${
        props.val ? "bg-black" : "bg-gray-200"
      } relative inline-flex h-6 w-11 items-center rounded-full`}
    >
      <span className="sr-only">Enable notifications</span>
      <span
        className={`${
          props.val ? "translate-x-6" : "translate-x-1"
        } inline-block h-4 w-4 transform rounded-full bg-white transition`}
      />
    </SwitchI>
  );
  */
};

export default Switch;
