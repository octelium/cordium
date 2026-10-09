import * as fs from "node:fs";
import * as path from "node:path";
import { pipeline } from "node:stream/promises";
import { Transform, type Readable } from "node:stream";
import {
  API_PREFIX,
  type ArtifactInfo,
  type FileInfo,
} from "../protocol/index.ts";
import { isValidID, newID, writeFileAtomic } from "./conversations.ts";

const mimeTypes: Record<string, string> = {
  ".txt": "text/plain",
  ".log": "text/plain",
  ".md": "text/markdown",
  ".csv": "text/csv",
  ".tsv": "text/tab-separated-values",
  ".json": "application/json",
  ".jsonl": "application/x-ndjson",
  ".ndjson": "application/x-ndjson",
  ".yaml": "application/yaml",
  ".yml": "application/yaml",
  ".xml": "application/xml",
  ".html": "text/html",
  ".htm": "text/html",
  ".css": "text/css",
  ".js": "text/javascript",
  ".ts": "text/plain",
  ".py": "text/x-python",
  ".go": "text/plain",
  ".sh": "text/x-shellscript",
  ".sql": "application/sql",
  ".diff": "text/x-diff",
  ".patch": "text/x-diff",
  ".png": "image/png",
  ".jpg": "image/jpeg",
  ".jpeg": "image/jpeg",
  ".gif": "image/gif",
  ".webp": "image/webp",
  ".svg": "image/svg+xml",
  ".pdf": "application/pdf",
  ".zip": "application/zip",
  ".gz": "application/gzip",
  ".tgz": "application/gzip",
  ".tar": "application/x-tar",
  ".parquet": "application/vnd.apache.parquet",
  ".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
  ".docx":
    "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
};

export const guessMimeType = (name: string): string =>
  mimeTypes[path.extname(name).toLowerCase()] ?? "application/octet-stream";

export const sanitizeFileName = (name: string): string => {
  const base = path.basename(name.replace(/\\/g, "/")).normalize("NFC");
  const cleaned = base
    .replace(/[\u0000-\u001f\u007f<>:"/\\|?*]/g, "_")
    .replace(/^\.+/, "_")
    .slice(0, 200)
    .trim();
  return cleaned === "" ? "file" : cleaned;
};

export class FileTooLargeError extends Error {}

export interface StoredArtifact {
  info: ArtifactInfo;
  filePath: string;
  conversationId?: string;
}

export class FileStore {
  readonly uploadsDir: string;
  readonly artifactsDir: string;

  constructor(dataDir: string) {
    this.uploadsDir = path.join(dataDir, "uploads");
    this.artifactsDir = path.join(dataDir, "artifacts");
  }

  init() {
    fs.mkdirSync(this.uploadsDir, { recursive: true, mode: 0o700 });
    fs.mkdirSync(this.artifactsDir, { recursive: true, mode: 0o700 });
  }

  async saveUpload(
    name: string,
    mimeType: string | undefined,
    source: Readable,
    maxBytes: number,
  ): Promise<FileInfo> {
    const id = newID();
    const fileName = sanitizeFileName(name);
    const dir = path.join(this.uploadsDir, id);
    fs.mkdirSync(dir, { recursive: true, mode: 0o700 });
    const filePath = path.join(dir, fileName);

    let size = 0;
    const counter = new Transform({
      transform(chunk: Buffer, _encoding, callback) {
        size += chunk.length;
        if (size > maxBytes) {
          callback(
            new FileTooLargeError(
              `The file exceeds the maximum allowed size of ${maxBytes} bytes`,
            ),
          );
          return;
        }
        callback(null, chunk);
      },
    });

    try {
      await pipeline(
        source,
        counter,
        fs.createWriteStream(filePath, { mode: 0o600 }),
      );
    } catch (err) {
      fs.rmSync(dir, { recursive: true, force: true });
      throw err;
    }

    const info: FileInfo = {
      id,
      name: fileName,
      mimeType:
        mimeType && mimeType !== "application/octet-stream"
          ? mimeType
          : guessMimeType(fileName),
      size,
      path: filePath,
      createdAt: new Date().toISOString(),
    };
    writeFileAtomic(path.join(dir, ".meta.json"), JSON.stringify(info));
    return info;
  }

  getUpload(id: string): FileInfo | undefined {
    if (!isValidID(id)) {
      return undefined;
    }
    try {
      return JSON.parse(
        fs.readFileSync(path.join(this.uploadsDir, id, ".meta.json"), "utf8"),
      );
    } catch {
      return undefined;
    }
  }

  publishArtifact(
    sourcePath: string,
    opts: {
      name?: string;
      title?: string;
      description?: string;
      mimeType?: string;
      conversationId?: string;
    } = {},
  ): ArtifactInfo {
    const stat = fs.statSync(sourcePath);
    if (!stat.isFile()) {
      throw new Error(`Not a regular file: ${sourcePath}`);
    }

    const id = newID();
    const name = sanitizeFileName(opts.name ?? path.basename(sourcePath));
    const dir = path.join(this.artifactsDir, id);
    fs.mkdirSync(dir, { recursive: true, mode: 0o700 });
    const filePath = path.join(dir, name);
    fs.copyFileSync(sourcePath, filePath);

    const info: ArtifactInfo = {
      id,
      name,
      mimeType: opts.mimeType ?? guessMimeType(name),
      size: stat.size,
      title: opts.title,
      description: opts.description,
      createdAt: new Date().toISOString(),
      url: `${API_PREFIX}/artifacts/${id}/content`,
    };

    const stored: StoredArtifact = {
      info,
      filePath,
      conversationId: opts.conversationId,
    };
    writeFileAtomic(path.join(dir, ".meta.json"), JSON.stringify(stored));
    return info;
  }

  getArtifact(id: string): StoredArtifact | undefined {
    if (!isValidID(id)) {
      return undefined;
    }
    try {
      const stored: StoredArtifact = JSON.parse(
        fs.readFileSync(path.join(this.artifactsDir, id, ".meta.json"), "utf8"),
      );
      if (!fs.existsSync(stored.filePath)) {
        return undefined;
      }
      return stored;
    } catch {
      return undefined;
    }
  }
}
