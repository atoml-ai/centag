// lib/download.js
// Downloads (and verifies) the platform-specific centag-personal Go binary
// and config seed data from GitHub Releases.
//
// The npm package bundles static/ (Vue frontend). The tarball from GitHub
// contains: binary + static/ + config/. We extract only binary + config/
// to avoid duplicating static/.
//
// Asset name (aligned with scripts/release/build-artifacts.sh):
//   centag-personal-<goos>-<goarch>.tar.gz
//
// Overridable via:
//   CENTAG_MIRROR     base URL that replaces the GitHub release base
//   CENTAG_TOKEN      optional bearer token for private mirrors
//   CENTAG_SKIP_DOWNLOAD=1   skip entirely (binary must already exist)

import { createHash } from 'node:crypto';
import { existsSync, mkdirSync, readFileSync, writeFileSync, chmodSync, rmSync, cpSync, readdirSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { get as httpsGet } from 'node:https';
import { get as httpGet } from 'node:http';
import { spawnSync } from 'node:child_process';

const __dirname = dirname(fileURLToPath(import.meta.url));
const PKG_ROOT = join(__dirname, '..');

// npm package version ↔ GitHub release tag. Kept in lockstep.
export const VERSION = JSON.parse(readFileSync(join(PKG_ROOT, 'package.json'), 'utf8')).version;

// Map node platform/arch → GOOS-GOARCH naming used by build-artifacts.sh
export function platformKey(platform = process.platform, arch = process.arch) {
  const goos = { darwin: 'darwin', linux: 'linux', win32: 'windows' }[platform];
  const goarch = { x64: 'amd64', arm64: 'arm64' }[arch];
  if (!goos || !goarch) {
    throw new Error(`unsupported platform ${platform}/${arch} (supported: darwin/linux/win32 × x64/arm64)`);
  }
  return `${goos}-${goarch}`;
}

export function binaryName(key) {
  return key.startsWith('windows-') ? 'centag-personal.exe' : 'centag-personal';
}

export function assetName(key) {
  return `centag-personal-${key}.tar.gz`;
}

// Where the extracted files live once installed (inside this npm package, cached).
export function installedDir(key) {
  return join(PKG_ROOT, 'bin', 'vendor', key);
}

export function installedBinaryPath(key) {
  return join(installedDir(key), binaryName(key));
}

export function releaseBaseURL() {
  const mirror = process.env.CENTAG_MIRROR?.replace(/\/+$/, '');
  if (mirror) return `${mirror}`;
  return `https://github.com/atoml-ai/centag/releases/download/v${VERSION}`;
}

function httpGetFollow(url, headers = {}) {
  return new Promise((resolve, reject) => {
    const get = url.startsWith('https:') ? httpsGet : httpGet;
    const req = get(url, { headers }, (res) => {
      if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
        res.resume();
        return resolve(httpGetFollow(new URL(res.headers.location, url).toString(), headers));
      }
      if (res.statusCode !== 200) {
        res.resume();
        return reject(new Error(`HTTP ${res.statusCode} for ${url}`));
      }
      const chunks = [];
      res.on('data', (c) => chunks.push(c));
      res.on('end', () => resolve(Buffer.concat(chunks)));
    });
    req.on('error', reject);
    req.setTimeout(120000, () => req.destroy(new Error(`timeout fetching ${url}`)));
  });
}

async function fetchChecksums(baseURL) {
  const token = process.env.CENTAG_TOKEN;
  const headers = token ? { Authorization: `Bearer ${token}` } : {};
  const body = await httpGetFollow(`${baseURL}/checksums.txt`, headers);
  const map = {};
  for (const line of body.toString('utf8').split('\n')) {
    const m = line.trim().match(/^([0-9a-fA-F]{64})\s+(.+)$/);
    if (m) map[m[2].trim()] = m[1].toLowerCase();
  }
  return map;
}

function sha256(buf) {
  return createHash('sha256').update(buf).digest('hex');
}

function findFileRecursive(dir, name) {
  for (const ent of readdirSync(dir, { withFileTypes: true })) {
    const p = join(dir, ent.name);
    if (ent.isDirectory()) {
      const hit = findFileRecursive(p, name);
      if (hit) return hit;
    } else if (ent.name === name) {
      return p;
    }
  }
  return null;
}

function findDirRecursive(dir, name) {
  for (const ent of readdirSync(dir, { withFileTypes: true })) {
    const p = join(dir, ent.name);
    if (ent.isDirectory()) {
      if (ent.name === name) return p;
      const hit = findDirRecursive(p, name);
      if (hit) return hit;
    }
  }
  return null;
}

function extractTarGz(archivePath, destDir) {
  mkdirSync(destDir, { recursive: true });
  const r = spawnSync('tar', ['-xzf', archivePath, '-C', destDir], { encoding: 'utf8' });
  if (r.status !== 0) {
    throw new Error(`tar extract failed: ${r.stderr || r.stdout || `exit ${r.status}`}`);
  }
}

// Ensure the binary and config for this platform exist; download + verify if missing.
// Returns { binPath, configDir }.
export async function ensureBinary() {
  const key = platformKey();
  const dest = installedDir(key);
  const binPath = installedBinaryPath(key);

  if (existsSync(binPath)) {
    return { binPath, vendorDir: dest };
  }

  if (process.env.CENTAG_SKIP_DOWNLOAD === '1') {
    throw new Error(
      `binary not found at ${binPath} and CENTAG_SKIP_DOWNLOAD=1; ` +
        `install with the offline package or unset the flag`
    );
  }

  const baseURL = releaseBaseURL();
  const archive = assetName(key);
  const url = `${baseURL}/${archive}`;
  const name = binaryName(key);

  process.stderr.write(`centag: downloading ${archive} from ${baseURL}\n`);
  const token = process.env.CENTAG_TOKEN;
  const headers = token ? { Authorization: `Bearer ${token}` } : {};
  const buf = await httpGetFollow(url, headers);

  // Verify against checksums.txt when present.
  try {
    const sums = await fetchChecksums(baseURL);
    const want = sums[archive];
    if (want) {
      const got = sha256(buf);
      if (got !== want) {
        throw new Error(`checksum mismatch for ${archive}: want ${want}, got ${got}`);
      }
    }
  } catch (e) {
    if (e.message.startsWith('checksum mismatch')) throw e;
    process.stderr.write(`centag: warning: could not verify checksum (${e.message}); continuing\n`);
  }

  const work = join(tmpdir(), `centag-${key}-${process.pid}`);
  mkdirSync(work, { recursive: true });
  const tarPath = join(work, archive);
  writeFileSync(tarPath, buf);
  extractTarGz(tarPath, work);

  // Find the extracted directory (centag-personal-<goos>-<goarch>/)
  const stageName = `centag-personal-${key}`;
  let stageDir = join(work, stageName);
  if (!existsSync(stageDir)) {
    // Fallback: find directory matching centag-personal-*
    const entries = readdirSync(work, { withFileTypes: true });
    const stage = entries.find(e => e.isDirectory() && e.name.startsWith('centag-personal-'));
    if (stage) stageDir = join(work, stage.name);
  }
  if (!existsSync(stageDir)) {
    rmSync(work, { recursive: true, force: true });
    throw new Error(`archive ${archive} did not contain expected directory`);
  }

  // Extract binary
  const foundBin = findFileRecursive(stageDir, name);
  if (!foundBin) {
    rmSync(work, { recursive: true, force: true });
    throw new Error(`archive ${archive} did not contain ${name}`);
  }
  mkdirSync(dest, { recursive: true });
  cpSync(foundBin, binPath);
  chmodSync(binPath, 0o755);

  // Extract config/ (seed data, profiles)
  const configDir = findDirRecursive(stageDir, 'config');
  if (configDir) {
    const destConfig = join(dest, 'config');
    if (existsSync(destConfig)) rmSync(destConfig, { recursive: true, force: true });
    cpSync(configDir, destConfig, { recursive: true });
  }

  rmSync(work, { recursive: true, force: true });
  return { binPath, vendorDir: dest };
}

// CLI entry for `node lib/download.js` (used by postinstall / tests).
if (import.meta.url === `file://${process.argv[1]}`) {
  ensureBinary()
    .then(({ binPath }) => process.stderr.write(`centag: ready at ${binPath}\n`))
    .catch((e) => {
      process.stderr.write(`centag: ${e.message}\n`);
      process.exit(1);
    });
}
