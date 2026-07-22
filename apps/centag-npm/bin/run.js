#!/usr/bin/env node
// bin/run.js
// npm bin shim for centag.
// Locates (downloading on demand) the platform Go binary and execs it,
// preserving argv, stdio, signals and exit code as closely as possible.

import { spawn } from 'node:child_process';
import { existsSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { ensureBinary, installedBinaryPath, installedDir, platformKey } from '../lib/download.js';

const __dirname = dirname(fileURLToPath(import.meta.url));
const PKG_ROOT = join(__dirname, '..');

function fail(msg) {
  process.stderr.write(`centag: ${msg}\n`);
  process.exit(1);
}

async function main() {
  let bin;
  let vendorDir;
  try {
    // Try the pre-bundled offline binary first (centag-offline ships these).
    const offline = installedBinaryPath(platformKey());
    if (existsSync(offline)) {
      bin = offline;
      vendorDir = installedDir(platformKey());
    } else {
      const result = await ensureBinary();
      bin = result.binPath;
      vendorDir = result.vendorDir;
    }
  } catch (e) {
    fail(e.message);
  }

  // Set environment variables for the Go binary.
  const env = { ...process.env };
  env.CENTAG_EDITION = env.CENTAG_EDITION || 'personal';

  // static/ is bundled in the npm package (not in vendor dir).
  const staticPath = join(PKG_ROOT, 'static');
  if (existsSync(staticPath)) {
    env.STATIC_PATH = env.STATIC_PATH || staticPath;
  }

  // Project root: vendor dir contains config/ seed data.
  if (vendorDir) {
    env.PROJECT_ROOT = env.PROJECT_ROOT || vendorDir;
    const initdataPath = join(vendorDir, 'config', 'profiles', 'personal', 'initdata');
    if (existsSync(initdataPath)) {
      env.INITDATA_PATH = env.INITDATA_PATH || initdataPath;
    }
  }

  const child = spawn(bin, process.argv.slice(2), {
    stdio: 'inherit',
    windowsHide: false,
    env,
  });

  // Forward termination signals so the wrapped process behaves like a direct child.
  const forward = (sig) => {
    if (!child.killed) {
      try { child.kill(sig); } catch { /* ignore */ }
    }
  };
  process.on('SIGINT', () => forward('SIGINT'));
  process.on('SIGTERM', () => forward('SIGTERM'));
  if (process.platform !== 'win32') {
    process.on('SIGHUP', () => forward('SIGHUP'));
  }

  child.on('error', (e) => fail(`failed to start ${bin}: ${e.message}`));
  child.on('exit', (code, signal) => {
    if (signal) {
      // Mimic the shell: exit with 128 + signal number.
      process.exit(128 + Number(signal.toString().replace(/\D/g, '')) || 1);
    }
    process.exit(code ?? 0);
  });
}

main();
