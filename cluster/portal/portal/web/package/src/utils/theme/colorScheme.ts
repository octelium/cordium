import {
  MantineColorScheme,
  localStorageColorSchemeManager,
  useComputedColorScheme,
} from "@mantine/core";
import * as React from "react";

export const COLOR_SCHEME_STORAGE_KEY = "cordium.colorScheme";

export const DEFAULT_COLOR_SCHEME: MantineColorScheme = "light";

export const colorSchemeManager = localStorageColorSchemeManager({
  key: COLOR_SCHEME_STORAGE_KEY,
});

export const useThemeColorMeta = () => {
  const colorScheme = useComputedColorScheme("light");

  React.useEffect(() => {
    const meta = document.querySelector('meta[name="theme-color"]');
    if (!meta) return;
    const canvas = getComputedStyle(document.documentElement)
      .getPropertyValue("--app-canvas")
      .trim();
    if (canvas) meta.setAttribute("content", canvas);
  }, [colorScheme]);
};
