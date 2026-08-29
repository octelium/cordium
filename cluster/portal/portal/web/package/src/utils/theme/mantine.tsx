import {
  ActionIcon,
  Alert,
  Badge,
  Button,
  Card,
  createTheme,
  Drawer,
  Menu,
  Modal,
  MultiSelect,
  NumberInput,
  PasswordInput,
  SegmentedControl,
  Select,
  Switch,
  Tabs,
  TagsInput,
  Textarea,
  TextInput,
  Tooltip,
} from "@mantine/core";

const fontFamily = [
  "Ubuntu",
  "ui-sans-serif",
  "system-ui",
  "-apple-system",
  "BlinkMacSystemFont",
  '"Segoe UI"',
  "Roboto",
  '"Helvetica Neue"',
  "Arial",
  "sans-serif",
].join(",");

const fontFamilyMonospace = [
  '"Ubuntu Mono"',
  "ui-monospace",
  "SFMono-Regular",
  "Menlo",
  "Consolas",
  "monospace",
].join(",");

const inputClassNames = {
  label: "text-slate-700",
  description: "text-slate-500",
};

const theme = createTheme({
  fontFamily,
  fontFamilyMonospace,
  primaryColor: "dark",
  autoContrast: true,
  defaultRadius: "md",
  cursorType: "pointer",

  headings: {
    fontFamily,
    fontWeight: "700",
    sizes: {
      h1: { fontSize: "1.6rem", lineHeight: "1.25" },
      h2: { fontSize: "1.3rem", lineHeight: "1.3" },
      h3: { fontSize: "1.05rem", lineHeight: "1.35" },
      h4: { fontSize: "0.95rem", lineHeight: "1.4" },
    },
  },

  components: {
    Button: Button.extend({
      defaultProps: { radius: "md" },
      classNames: { root: "font-semibold" },
    }),
    ActionIcon: ActionIcon.extend({
      defaultProps: { radius: "md", variant: "subtle" },
    }),
    Badge: Badge.extend({
      defaultProps: { radius: "sm" },
      classNames: { label: "font-semibold normal-case tracking-normal" },
    }),
    Card: Card.extend({
      defaultProps: { radius: "lg", withBorder: true, padding: "lg" },
    }),
    TextInput: TextInput.extend({ classNames: inputClassNames }),
    Textarea: Textarea.extend({ classNames: inputClassNames }),
    NumberInput: NumberInput.extend({ classNames: inputClassNames }),
    PasswordInput: PasswordInput.extend({ classNames: inputClassNames }),
    TagsInput: TagsInput.extend({ classNames: inputClassNames }),
    Select: Select.extend({
      defaultProps: {
        comboboxProps: {
          shadow: "md",
          radius: "md",
          transitionProps: { transition: "pop", duration: 120 },
        },
      },
      classNames: { ...inputClassNames, option: "font-medium" },
    }),
    MultiSelect: MultiSelect.extend({
      defaultProps: {
        comboboxProps: {
          shadow: "md",
          radius: "md",
          transitionProps: { transition: "pop", duration: 120 },
        },
      },
      classNames: { ...inputClassNames, option: "font-medium" },
    }),
    Switch: Switch.extend({
      classNames: {
        label: "font-medium text-slate-700",
        description: "text-slate-500",
      },
    }),
    SegmentedControl: SegmentedControl.extend({
      defaultProps: { radius: "md" },
      classNames: { label: "font-semibold" },
    }),
    Tabs: Tabs.extend({
      classNames: { tab: "font-semibold" },
    }),
    Tooltip: Tooltip.extend({
      defaultProps: {
        withArrow: true,
        openDelay: 250,
        transitionProps: { transition: "fade", duration: 120 },
      },
      classNames: { tooltip: "text-xs font-medium" },
    }),
    Modal: Modal.extend({
      defaultProps: {
        centered: true,
        radius: "lg",
        overlayProps: { backgroundOpacity: 0.4, blur: 2 },
        transitionProps: { transition: "pop", duration: 140 },
      },
      classNames: { title: "font-bold text-slate-800" },
    }),
    Drawer: Drawer.extend({
      defaultProps: {
        position: "right",
        overlayProps: { backgroundOpacity: 0.4, blur: 2 },
      },
      classNames: { title: "font-bold text-slate-800" },
    }),
    Menu: Menu.extend({
      defaultProps: { shadow: "lg", radius: "md", width: 220 },
      classNames: { item: "font-medium" },
    }),
    Alert: Alert.extend({
      defaultProps: { radius: "md", variant: "light" },
      classNames: { title: "font-bold" },
    }),
  },
});

export default theme;
