import { strict as assert } from "node:assert";
import { execFileSync } from "node:child_process";
import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";
import { it } from "node:test";
import { fileURLToPath, pathToFileURL } from "node:url";

it(
  "installs and starts the packed agent with compatible Pi dependencies",
  { timeout: 180000 },
  async () => {
    const packageDir = fileURLToPath(new URL("..", import.meta.url));
    const dir = fs.mkdtempSync(
      path.join(os.tmpdir(), "cordium-agent-package-"),
    );
    try {
      const [packed] = Object.values(
        JSON.parse(
          execFileSync("npm", ["pack", "--json", "--pack-destination", dir], {
            cwd: packageDir,
            encoding: "utf8",
            timeout: 30000,
          }),
        ),
      );
      fs.writeFileSync(
        path.join(dir, "package.json"),
        JSON.stringify({ private: true }),
      );
      execFileSync(
        "npm",
        [
          "install",
          "--omit=dev",
          "--no-audit",
          "--no-fund",
          path.join(dir, packed.filename),
        ],
        { cwd: dir, timeout: 120000 },
      );

      const installedDir = path.join(
        dir,
        "node_modules/@octelium/cordium-agent",
      );
      const pkg = JSON.parse(
        fs.readFileSync(path.join(installedDir, "package.json"), "utf8"),
      );
      assert.equal(
        execFileSync(
          process.execPath,
          [path.join(installedDir, "dist/cli.js"), "version"],
          {
            cwd: dir,
            encoding: "utf8",
            timeout: 30000,
          },
        ),
        `${pkg.version}\n`,
      );
      const tree = JSON.parse(
        execFileSync("npm", ["ls", "--all", "--omit=dev", "--json"], {
          cwd: dir,
          encoding: "utf8",
          timeout: 30000,
        }),
      );
      const checkDependencies = (dependencies = {}) => {
        for (const [name, dependency] of Object.entries(dependencies)) {
          if (name.startsWith("@earendil-works/")) {
            assert.equal(dependency.version, pkg.dependencies[name], name);
          }
          checkDependencies(dependency.dependencies);
        }
      };
      checkDependencies(tree.dependencies);

      const { createApp, resolveConfig } = await import(
        pathToFileURL(path.join(installedDir, "dist/index.js")).href
      );
      const app = await createApp({
        config: resolveConfig(
          {
            dataDir: path.join(dir, "data"),
            workDir: path.join(dir, "work"),
            logLevel: "error",
            server: { host: "127.0.0.1", port: 0 },
            octelium: { mode: "disabled" },
            llm: {
              provider: "custom",
              baseUrl: "http://127.0.0.1:1",
              model: "package-test",
            },
            skills: { enabled: false },
          },
          {},
        ),
      });
      try {
        const address = await app.start();
        const response = await fetch(
          `http://127.0.0.1:${address.port}/healthz`,
        );
        assert.equal(response.status, 200);
      } finally {
        await app.close();
      }
    } finally {
      fs.rmSync(dir, { recursive: true, force: true });
    }
  },
);
