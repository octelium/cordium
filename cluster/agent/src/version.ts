import * as fs from "node:fs";

export const getVersion = (): string => {
  try {
    const pkg = JSON.parse(
      fs.readFileSync(new URL("../package.json", import.meta.url), "utf8"),
    );
    return typeof pkg.version === "string" ? pkg.version : "unknown";
  } catch {
    return "unknown";
  }
};
