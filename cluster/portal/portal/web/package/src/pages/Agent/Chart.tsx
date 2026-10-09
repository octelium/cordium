import type { ChartSpec } from "@/apis/agent/protocol";
import { useComputedColorScheme } from "@mantine/core";
import { BarChart, LineChart, PieChart, ScatterChart } from "echarts/charts";
import {
  AriaComponent,
  GridComponent,
  LegendComponent,
  TooltipComponent,
} from "echarts/components";
import * as echarts from "echarts/core";
import { CanvasRenderer } from "echarts/renderers";
import ReactEChartsCore from "echarts-for-react/esm/core";
import * as React from "react";
import { formatValue } from "./utils";

echarts.use([
  AriaComponent,
  TooltipComponent,
  GridComponent,
  LegendComponent,
  BarChart,
  LineChart,
  PieChart,
  ScatterChart,
  CanvasRenderer,
]);

const paletteVars = [
  "--app-hue-blue-solid",
  "--app-hue-emerald-solid",
  "--app-hue-amber-solid",
  "--app-hue-violet-solid",
  "--app-hue-rose-solid",
  "--app-hue-sky-solid",
  "--app-hue-teal-solid",
];

const toRGB = (() => {
  let ctx: CanvasRenderingContext2D | null | undefined;
  return (color: string, fallback: string): string => {
    if (!color) return fallback;
    ctx ??= document.createElement("canvas").getContext("2d", {
      willReadFrequently: true,
    });
    if (!ctx) return fallback;
    ctx.clearRect(0, 0, 1, 1);
    ctx.fillStyle = fallback;
    ctx.fillStyle = color;
    ctx.fillRect(0, 0, 1, 1);
    const [r, g, b, a] = ctx.getImageData(0, 0, 1, 1).data;
    return `rgba(${r}, ${g}, ${b}, ${(a / 255).toFixed(3)})`;
  };
})();

interface ChartTheme {
  palette: string[];
  ink: string;
  muted: string;
  grid: string;
  surface: string;
}

const readTheme = (_scheme: string): ChartTheme => {
  const style = getComputedStyle(document.documentElement);
  const v = (name: string, fallback: string) =>
    toRGB(style.getPropertyValue(name).trim(), fallback);
  return {
    palette: paletteVars.map((name) => v(name, "#3b82f6")),
    ink: v("--app-ink", "#0f172a"),
    muted: v("--app-ink-muted", "#64748b"),
    grid: v("--app-line", "#e2e8f0"),
    surface: v("--app-surface", "#ffffff"),
  };
};

const toNumber = (value: unknown): number | null => {
  if (value === null || value === undefined || value === "") return null;
  const ret = typeof value === "number" ? value : Number(value);
  return Number.isFinite(ret) ? ret : null;
};

const buildChartOption = (spec: ChartSpec, theme: ChartTheme) => {
  const textStyle = {
    color: theme.muted,
    fontSize: 11,
    fontWeight: 600,
    fontFamily: "Ubuntu, ui-sans-serif, system-ui, sans-serif",
  };
  const tooltip = {
    confine: true,
    backgroundColor: theme.surface,
    borderColor: theme.grid,
    borderWidth: 1,
    textStyle: { ...textStyle, color: theme.ink, fontSize: 12 },
    extraCssText:
      "border-radius: 10px; box-shadow: 0 8px 24px rgb(0 0 0 / 0.12);",
  };
  const legend = {
    show: spec.series.length > 1 || spec.chartType === "pie",
    type: "scroll",
    top: 0,
    icon: "roundRect",
    itemWidth: 9,
    itemHeight: 9,
    textStyle,
    pageTextStyle: textStyle,
  };

  if (spec.chartType === "pie") {
    const series = spec.series[0];
    return {
      color: theme.palette,
      tooltip: { ...tooltip, trigger: "item" },
      legend,
      series: [
        {
          type: "pie",
          radius: ["44%", "72%"],
          top: 24,
          itemStyle: {
            borderColor: theme.surface,
            borderWidth: 2,
            borderRadius: 4,
          },
          label: { ...textStyle, formatter: "{b}: {d}%" },
          data: spec.rows.map((row) => ({
            name: formatValue(row[spec.x.key]),
            value: toNumber(row[series?.key ?? ""]) ?? 0,
          })),
        },
      ],
    };
  }

  const xType =
    spec.x.type === "time"
      ? "time"
      : spec.x.type === "number"
        ? "value"
        : "category";

  const xValue = (value: unknown) =>
    xType === "time"
      ? new Date(String(value)).getTime()
      : xType === "value"
        ? toNumber(value)
        : formatValue(value);

  return {
    color: theme.palette,
    aria: { enabled: true, decal: { show: false } },
    grid: {
      top: legend.show ? 34 : 16,
      right: 14,
      bottom: 24,
      left: 8,
      containLabel: true,
    },
    legend,
    tooltip: {
      ...tooltip,
      trigger: spec.chartType === "scatter" ? "item" : "axis",
    },
    xAxis: {
      type: xType,
      name: spec.x.label,
      nameLocation: "middle",
      nameGap: 28,
      nameTextStyle: textStyle,
      data:
        xType === "category"
          ? spec.rows.map((row) => formatValue(row[spec.x.key]))
          : undefined,
      axisLabel: { ...textStyle, hideOverlap: true },
      axisLine: { lineStyle: { color: theme.grid } },
      axisTick: { show: false },
      splitLine: { show: false },
    },
    yAxis: {
      type: "value",
      name: spec.y?.label ?? spec.y?.unit,
      nameTextStyle: textStyle,
      axisLabel: textStyle,
      splitLine: { lineStyle: { color: theme.grid, type: "dashed" } },
    },
    series: spec.series.map((series) => ({
      name: series.label ?? series.key,
      type: spec.chartType === "area" ? "line" : spec.chartType,
      stack: spec.stacked ? "total" : undefined,
      smooth: spec.chartType === "line" || spec.chartType === "area",
      showSymbol: spec.chartType === "scatter" || spec.rows.length < 40,
      areaStyle: spec.chartType === "area" ? { opacity: 0.18 } : undefined,
      barMaxWidth: 28,
      itemStyle:
        spec.chartType === "bar" ? { borderRadius: [4, 4, 0, 0] } : undefined,
      data:
        xType === "category"
          ? spec.rows.map((row) => toNumber(row[series.key]))
          : spec.rows.map((row) => [
              xValue(row[spec.x.key]),
              toNumber(row[series.key]),
            ]),
    })),
  };
};

const Chart = (props: { chart: ChartSpec }) => {
  const scheme = useComputedColorScheme("light");
  const option = React.useMemo(
    () => buildChartOption(props.chart, readTheme(scheme)),
    [props.chart, scheme],
  );

  return (
    <ReactEChartsCore
      echarts={echarts}
      option={option}
      notMerge
      style={{ height: 280, width: "100%" }}
    />
  );
};

export default Chart;
