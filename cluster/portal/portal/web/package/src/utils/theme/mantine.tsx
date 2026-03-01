import {
  Accordion,
  Button,
  createTheme,
  MultiSelect,
  NumberInput,
  Select,
  Switch,
  Tabs,
  TagsInput,
  Textarea,
  TextInput,
  Tooltip,
} from "@mantine/core";

// import { fontFamily } from ".";

const inputClassName =
  "!font-bold !focus:shadow-md !transition-all !duration-500 !rounded-md !focus:border-gray-900 !border-[2px]";

const theme = createTheme({
  // fontFamily: fontFamily,
  fontFamily: [
    "-apple-system",
    "BlinkMacSystemFont",
    "Ubuntu",
    '"Segoe UI"',
    "Roboto",
    '"Helvetica Neue"',
    "Arial",
    "sans-serif",
    '"Apple Color Emoji"',
    '"Segoe UI Emoji"',
    '"Segoe UI Symbol"',
  ].join(","),

  primaryColor: "dark",
  autoContrast: true,
  defaultRadius: "md",
  // focusRing: "never",

  components: {
    Button: Button.extend({
      defaultProps: {
        variant: "filled",
        className:
          "!font-bold !shadow-md !transition-all !duration-500 !rounded-md",
      },
    }),
    TextInput: TextInput.extend({
      classNames: {
        label: "!font-bold",
        input: inputClassName,
      },
    }),
    Textarea: Textarea.extend({
      classNames: {
        label: "!font-bold",
        input: inputClassName,
      },
    }),
    NumberInput: NumberInput.extend({
      classNames: {
        label: "!font-bold",
        input: inputClassName,
      },
    }),
    TagsInput: TagsInput.extend({
      classNames: {
        label: "!font-bold",
        input: inputClassName,
      },
    }),
    Accordion: Accordion.extend({
      classNames: {
        label: "!font-bold",
        panel: "!font-bold",
      },
    }),
    Tabs: Tabs.extend({
      classNames: {
        tab: "!font-bold",
        panel: "!font-bold",
      },
    }),
    Switch: Switch.extend({
      defaultProps: {
        // size: "md",
      },
      classNames: {
        label: "!font-bold",
        input: "!transition-all !duration-500",
      },
    }),
    Select: Select.extend({
      defaultProps: {
        radius: "md",
        comboboxProps: {
          transitionProps: { transition: "pop", duration: 200 },
          shadow: "sm",
          radius: "md",
        },
      },
      classNames: {
        input: inputClassName,
        label: "!font-bold",
        option: "!transition-all !duration-500 !font-bold !hover:bg-zinc-200",
      },
    }),
    MultiSelect: MultiSelect.extend({
      defaultProps: {
        radius: "md",
        comboboxProps: {
          transitionProps: { transition: "pop", duration: 200 },
          shadow: "sm",
          radius: "md",
        },
      },
      classNames: {
        input: inputClassName,
        label: "!font-bold",
        option: "!transition-all !duration-500 !font-bold !hover:bg-zinc-200",
      },
    }),
    Tooltip: Tooltip.extend({
      defaultProps: {
        transitionProps: {
          transition: "fade",
          duration: 350,
        },
        classNames: {
          tooltip: "!shadow-md !font-bold !text-xs !rounded-sm",
        },
      },
    }),
  },
});

export default theme;
