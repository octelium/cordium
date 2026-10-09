import * as fs from "node:fs";
import * as path from "node:path";
import {
  APPROVAL_MODES,
  type ApprovalMode,
  type ApprovalSettings,
} from "../protocol/index.ts";
import { writeFileAtomic } from "./conversations.ts";

export interface StoredSettings {
  model?: {
    provider: string;
    id: string;
  };
  thinkingLevel?: string;
  approvals?: Partial<ApprovalSettings>;
}

const isObject = (arg: unknown): arg is Record<string, unknown> =>
  typeof arg === "object" && arg !== null && !Array.isArray(arg);

export class SettingsStore {
  private filePath: string;

  constructor(dataDir: string) {
    this.filePath = path.join(dataDir, "settings.json");
  }

  read(): StoredSettings {
    let parsed: unknown;
    try {
      parsed = JSON.parse(fs.readFileSync(this.filePath, "utf8"));
    } catch {
      return {};
    }
    if (!isObject(parsed)) {
      return {};
    }

    const ret: StoredSettings = {};
    const model = parsed.model;
    if (
      isObject(model) &&
      typeof model.provider === "string" &&
      typeof model.id === "string"
    ) {
      ret.model = { provider: model.provider, id: model.id };
    }
    if (typeof parsed.thinkingLevel === "string") {
      ret.thinkingLevel = parsed.thinkingLevel;
    }
    const approvals = parsed.approvals;
    if (isObject(approvals)) {
      ret.approvals = {};
      if (APPROVAL_MODES.includes(approvals.api as ApprovalMode)) {
        ret.approvals.api = approvals.api as ApprovalMode;
      }
      if (typeof approvals.commands === "boolean") {
        ret.approvals.commands = approvals.commands;
      }
    }
    return ret;
  }

  write(settings: StoredSettings) {
    fs.mkdirSync(path.dirname(this.filePath), { recursive: true, mode: 0o700 });
    writeFileAtomic(this.filePath, JSON.stringify(settings, null, 2));
  }

  update(patch: Partial<StoredSettings>): StoredSettings {
    const ret = { ...this.read(), ...patch };
    for (const key of Object.keys(ret) as (keyof StoredSettings)[]) {
      if (ret[key] === undefined) {
        delete ret[key];
      }
    }
    this.write(ret);
    return ret;
  }
}
