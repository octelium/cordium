import { resourceKey, type ResourceRef } from "../protocol/index.ts";

const isRecord = (arg: unknown): arg is Record<string, unknown> =>
  typeof arg === "object" && arg !== null && !Array.isArray(arg);

export const toResourceRef = (arg: unknown): ResourceRef | undefined => {
  if (!isRecord(arg)) {
    return undefined;
  }
  const { apiVersion, kind, metadata } = arg;
  if (
    typeof apiVersion !== "string" ||
    typeof kind !== "string" ||
    !isRecord(metadata) ||
    typeof metadata.name !== "string" ||
    metadata.name === ""
  ) {
    return undefined;
  }
  return {
    apiVersion,
    kind,
    name: metadata.name,
    uid: typeof metadata.uid === "string" ? metadata.uid : undefined,
  };
};

export const extractResources = (
  json: unknown,
  limit = 200,
): { refs: ResourceRef[]; objects: Record<string, unknown>[] } => {
  const refs: ResourceRef[] = [];
  const objects: Record<string, unknown>[] = [];

  const walk = (value: unknown, depth: number) => {
    if (refs.length >= limit || depth > 6) {
      return;
    }
    if (Array.isArray(value)) {
      for (const item of value) {
        walk(item, depth + 1);
      }
      return;
    }
    if (!isRecord(value)) {
      return;
    }
    const ref = toResourceRef(value);
    if (ref) {
      refs.push(ref);
      objects.push(value);
      return;
    }
    for (const child of Object.values(value)) {
      walk(child, depth + 1);
    }
  };

  walk(json, 0);
  return { refs, objects };
};

export class ResourceCache {
  private items = new Map<string, Record<string, unknown>>();
  private max: number;

  constructor(max = 500) {
    this.max = max;
  }

  remember(objects: Record<string, unknown>[]) {
    for (const obj of objects) {
      const ref = toResourceRef(obj);
      if (!ref) {
        continue;
      }
      const key = resourceKey(ref);
      this.items.delete(key);
      this.items.set(key, obj);
      if (this.items.size > this.max) {
        const oldest = this.items.keys().next().value;
        if (oldest !== undefined) {
          this.items.delete(oldest);
        }
      }
    }
  }

  get(ref: ResourceRef): Record<string, unknown> | undefined {
    return this.items.get(resourceKey(ref));
  }
}
