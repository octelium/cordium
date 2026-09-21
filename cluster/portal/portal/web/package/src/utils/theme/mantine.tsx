import {
  ActionIcon,
  Alert,
  Badge,
  Button,
  Card,
  CSSVariablesResolver,
  createTheme,
  Drawer,
  MantineColorsTuple,
  Menu,
  Modal,
  MultiSelect,
  NumberInput,
  Pagination,
  PasswordInput,
  SegmentedControl,
  Select,
  Switch,
  Tabs,
  TagsInput,
  Textarea,
  TextInput,
  Tooltip,
  virtualColor,
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

const zinc: MantineColorsTuple = [
  "#fafafa",
  "#f4f4f5",
  "#e4e4e7",
  "#d4d4d8",
  "#a1a1aa",
  "#71717a",
  "#52525b",
  "#3f3f46",
  "#27272a",
  "#18181b",
];

export const cssVariablesResolver: CSSVariablesResolver = () => ({
  variables: {},
  light: {},
  dark: {
    "--mantine-color-dark-0": "var(--app-ink)",
    "--mantine-color-dark-1": "var(--app-ink-strong)",
    "--mantine-color-dark-2": "var(--app-ink-muted)",
    "--mantine-color-dark-3": "var(--app-ink-subtle)",
    "--mantine-color-dark-4": "var(--app-line)",
    "--mantine-color-dark-5": "var(--app-surface-strong)",
    "--mantine-color-dark-6": "var(--app-surface)",
    "--mantine-color-dark-7": "var(--app-canvas)",
    "--mantine-color-dark-8": "var(--app-canvas)",
    "--mantine-color-dark-9": "var(--app-canvas)",

    "--mantine-color-body": "var(--app-surface)",
    "--mantine-color-text": "var(--app-ink)",
    "--mantine-color-dimmed": "var(--app-ink-muted)",
    "--mantine-color-placeholder": "var(--app-ink-subtle)",
    "--mantine-color-anchor": "var(--app-ink)",
    "--mantine-color-default": "var(--app-surface)",
    "--mantine-color-default-hover": "var(--app-surface-hover)",
    "--mantine-color-default-color": "var(--app-ink)",
    "--mantine-color-default-border": "var(--app-line)",
    "--mantine-color-disabled": "var(--app-surface-subtle)",
    "--mantine-color-disabled-color": "var(--app-ink-faint)",
    "--mantine-color-disabled-border": "var(--app-line-subtle)",

    "--mantine-color-primary-filled": "var(--app-inverted)",
    "--mantine-color-primary-filled-hover": "var(--app-inverted-hover)",
    "--mantine-color-primary-contrast": "var(--app-on-inverted)",
    "--mantine-primary-color-contrast": "var(--app-on-inverted)",
    "--mantine-color-primary-light": "var(--app-surface-hover)",
    "--mantine-color-primary-light-hover": "var(--app-surface-strong)",
    "--mantine-color-primary-light-color": "var(--app-ink)",
    "--mantine-color-primary-outline": "var(--app-line-strong)",
    "--mantine-color-primary-outline-hover": "var(--app-surface-hover)",
    "--mantine-color-primary-text": "var(--app-ink)",

    "--mantine-color-red-light": "var(--app-hue-rose-soft)",
    "--mantine-color-red-light-hover": "var(--app-hue-rose-line)",
    "--mantine-color-red-light-color": "var(--app-hue-rose)",
    "--mantine-color-gray-light": "var(--app-surface-hover)",
    "--mantine-color-gray-light-hover": "var(--app-surface-strong)",
    "--mantine-color-gray-light-color": "var(--app-ink-body)",
  },
});

const inputClassNames = {
  label: "text-ink-body",
  description: "text-ink-muted",
};

const theme = createTheme({
  fontFamily,
  fontFamilyMonospace,
  colors: {
    zinc,
    primary: virtualColor({ name: "primary", light: "dark", dark: "zinc" }),
  },
  primaryColor: "primary",
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
      vars: () => ({ root: { "--switch-color": "var(--app-switch-on)" } }),
      classNames: {
        label: "font-medium text-ink-body",
        description: "text-ink-muted",
      },
    }),
    SegmentedControl: SegmentedControl.extend({
      defaultProps: { radius: "md" },
      classNames: { label: "font-semibold" },
    }),
    Tabs: Tabs.extend({
      defaultProps: { autoContrast: false },
      classNames: {
        root: "[--tabs-text-color:var(--mantine-primary-color-contrast)]",
        tab: "font-semibold",
      },
    }),
    Pagination: Pagination.extend({
      vars: () => ({
        root: {
          "--pagination-active-color": "var(--mantine-primary-color-contrast)",
        },
      }),
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
      classNames: { title: "font-bold text-ink-strong" },
    }),
    Drawer: Drawer.extend({
      defaultProps: {
        position: "right",
        overlayProps: { backgroundOpacity: 0.4, blur: 2 },
      },
      classNames: { title: "font-bold text-ink-strong" },
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
