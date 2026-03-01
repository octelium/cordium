import { toNumOrZero } from "@/utils";
import { NumberInput, Textarea, TextInput } from "@mantine/core";

const Field = (props: {
  val: string | number;
  onChange: (val: string | number) => void;
  isRequired?: boolean;
  label: string;
  multiLine?: boolean;
  rows?: number;
  maxRows?: number;
  isNumber?: boolean;
  placeholder?: string;
  description?: string;
}) => {
  if (props.rows) {
    return (
      <Textarea
        value={props.val}
        rows={props.rows}
        required={props.isRequired}
        onChange={(e) => {
          props.onChange(e.target.value);
        }}
        label={props.label}
        placeholder={props.placeholder}
        description={props.description}

        /*
        className={twMerge(
          "inline-flex w-full outline-none p-2 border-[3px]",
          "rounded-lg border-gray-400 focus:border-gray-900",
          "transition-all duration-300 focus:shadow-md",
          "resize-none",
          "font-semibold"
        )}
        */
      />
    );
  }

  if (props.isNumber) {
    <NumberInput
      value={props.val}
      onChange={(e) => {
        props.onChange(e as number);
      }}
      label={props.label}
      required={props.isRequired}
      placeholder={props.placeholder}
      description={props.description}
      min={0}
      /*
      className={twMerge(
        "inline-flex w-full outline-none p-2 border-[3px]",
        "rounded-lg border-gray-400 focus:border-gray-900",
        "transition-all duration-300 focus:shadow-md",
        "font-semibold"
      )}
      */
    />;
  }
  return (
    <TextInput
      value={props.val}
      required={props.isRequired}
      onChange={(e) => {
        if (props.isNumber && e.target.value == "") {
          props.onChange(0);
          return;
        } else if (props.isNumber) {
          props.onChange(toNumOrZero(e.target.value) ?? 0);
          return;
        }

        props.onChange(e.target.value);
      }}
      label={props.label}
      placeholder={props.placeholder}
      description={props.description}
      className="font-semibold transition-all duration-500"
      /*
      className={twMerge(
        "inline-flex w-full outline-none p-2 border-[3px]",
        "rounded-lg border-gray-400 focus:border-gray-900",
        "transition-all duration-300 focus:shadow-md",
        "font-semibold"
      )}
      */
    />
  );

  /*
  return (
    <TextField
      defaultValue={props.val}
      variant="outlined"
      size="small"
      required={props.isRequired}
      fullWidth={true}
      multiline={props.multiLine}
      label={props.label}
      rows={props.rows}
      maxRows={props.rows}
      type={props.isNumber ? "number" : undefined}
      onChange={(e) => {
        props.onChange(e.target.value);
      }}
    />
  );
  */
};

export default Field;
