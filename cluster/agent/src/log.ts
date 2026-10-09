export type LogLevel = "debug" | "info" | "warn" | "error";

const levelOrder: Record<LogLevel, number> = {
  debug: 10,
  info: 20,
  warn: 30,
  error: 40,
};

export type LogFields = Record<string, unknown>;

export interface Logger {
  debug(msg: string, fields?: LogFields): void;
  info(msg: string, fields?: LogFields): void;
  warn(msg: string, fields?: LogFields): void;
  error(msg: string, fields?: LogFields): void;
  child(fields: LogFields): Logger;
}

let currentLevel: LogLevel = "info";

export const setLogLevel = (level: LogLevel) => {
  currentLevel = level;
};

const formatValue = (value: unknown): string => {
  if (value instanceof Error) {
    return JSON.stringify(value.message);
  }
  if (typeof value === "string") {
    return /^[\w./:@-]*$/.test(value) && value !== ""
      ? value
      : JSON.stringify(value);
  }
  return JSON.stringify(value) ?? "undefined";
};

const write = (
  level: LogLevel,
  msg: string,
  base: LogFields,
  fields?: LogFields,
) => {
  if (levelOrder[level] < levelOrder[currentLevel]) {
    return;
  }

  const all = { ...base, ...fields };
  const suffix = Object.entries(all)
    .filter(([, value]) => value !== undefined)
    .map(([key, value]) => `${key}=${formatValue(value)}`)
    .join(" ");

  process.stderr.write(
    `${new Date().toISOString()} ${level.toUpperCase()} ${msg}${suffix ? ` ${suffix}` : ""}\n`,
  );
};

const createLogger = (base: LogFields): Logger => ({
  debug: (msg, fields) => write("debug", msg, base, fields),
  info: (msg, fields) => write("info", msg, base, fields),
  warn: (msg, fields) => write("warn", msg, base, fields),
  error: (msg, fields) => write("error", msg, base, fields),
  child: (fields) => createLogger({ ...base, ...fields }),
});

export const logger = createLogger({});
