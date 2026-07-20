#!/usr/bin/env node
// bin/run.js
// npm bin shim for centag-proxyctl.
// Locates (downloading on demand) the platform Go binary and execs it,
// preserving argv, stdio, signals and exit code as closely as possible.

import { spawn } from 'node:child_process';
import { existsSync } from 'node:fs';
import { ensureBinary, installedBinaryPath, platformKey } from '../lib/download.js';

function fail(msg) {
  process.stderr.write(`centag-proxyctl: ${msg}\n`);
  process.exit(1);
}

async function main() {
  let bin;
  try {
    // Try the pre-bundled offline binary first (centag-proxyctl-offline ships these).
    const offline = installedBinaryPath(platformKey());
    if (existsSync(offline)) {
      bin = offline;
    } else {
      bin = await ensureBinary();
    }
  } catch (e) {
    fail(e.message);
  }

  const child = spawn(bin, process.argv.slice(2), {
    stdio: 'inherit',
    windowsHide: false,
  });

  // Forward termination signals so the wrapped agent behaves like a direct child.
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
