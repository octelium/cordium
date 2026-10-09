import * as fs from "node:fs";
import { pathToFileURL } from "node:url";
import semver from "semver";

export const resolvePublishVersion = ({
  eventName,
  refType,
  refName,
  release,
  inputs = {},
  baseVersion,
  runNumber,
  runAttempt,
}) => {
  const isManual = eventName === "workflow_dispatch";
  const isRelease = eventName === "release" || refType === "tag";
  let version;
  let tag;

  if (isManual && inputs.version) {
    version = inputs.version.replace(/^v/, "");
  } else if (isRelease) {
    version = (release?.tag_name ?? refName).replace(/^v/, "");
  }

  if (isManual) {
    tag = inputs.tag;
  } else if (isRelease) {
    tag = release?.prerelease || semver.prerelease(version) ? "next" : "latest";
  } else if (eventName === "push" && refType === "branch") {
    tag = refName;
  } else {
    throw new Error(`Unsupported publishing event: ${eventName}`);
  }

  if (
    typeof tag !== "string" ||
    !tag ||
    tag.startsWith("-") ||
    tag !== tag.trim() ||
    encodeURIComponent(tag) !== tag ||
    semver.validRange(tag)
  ) {
    throw new Error(`Invalid npm dist-tag: ${tag}`);
  }

  if (version === undefined) {
    const base = semver.parse(baseVersion);
    if (!base) throw new Error(`Invalid package version: ${baseVersion}`);
    if (!/^[1-9][0-9]*$/.test(String(runNumber))) {
      throw new Error(`Invalid workflow run number: ${runNumber}`);
    }
    if (!/^[1-9][0-9]*$/.test(String(runAttempt))) {
      throw new Error(`Invalid workflow run attempt: ${runAttempt}`);
    }
    const prerelease = tag.replace(/[^0-9A-Za-z-]/g, "-");
    version = `${base.major}.${base.minor}.${base.patch}-${prerelease}.${runNumber}.${runAttempt}`;
  }

  if (semver.valid(version) !== version) {
    throw new Error(`Invalid publishing version: ${version}`);
  }
  return { version, tag };
};

if (
  process.argv[1] &&
  pathToFileURL(process.argv[1]).href === import.meta.url
) {
  const event = JSON.parse(
    fs.readFileSync(process.env.GITHUB_EVENT_PATH, "utf8"),
  );
  const pkg = JSON.parse(
    fs.readFileSync(new URL("../package.json", import.meta.url), "utf8"),
  );
  const { version, tag } = resolvePublishVersion({
    eventName: process.env.GITHUB_EVENT_NAME,
    refType: process.env.GITHUB_REF_TYPE,
    refName: process.env.GITHUB_REF_NAME,
    release: event.release,
    inputs: event.inputs,
    baseVersion: pkg.version,
    runNumber: process.env.GITHUB_RUN_NUMBER,
    runAttempt: process.env.GITHUB_RUN_ATTEMPT,
  });
  fs.appendFileSync(
    process.env.GITHUB_OUTPUT,
    `version=${version}\ntag=${tag}\n`,
  );
  console.log(`Publishing ${pkg.name}@${version} with npm tag ${tag}`);
}
