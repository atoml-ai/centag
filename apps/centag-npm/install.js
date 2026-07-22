// install.js
// npm postinstall: pre-download the platform binary so `npx centag`
// works offline after install. Failures are non-fatal (lazy download at runtime)
// unless CENTAG_REQUIRE_DOWNLOAD=1 is set.

import { ensureBinary } from './lib/download.js';

async function main() {
  if (process.env.CENTAG_SKIP_DOWNLOAD === '1') {
    process.stderr.write('centag: skip download (CENTAG_SKIP_DOWNLOAD=1)\n');
    return;
  }
  try {
    const { binPath } = await ensureBinary();
    process.stderr.write(`centag: installed at ${binPath}\n`);
  } catch (e) {
    const hard = process.env.CENTAG_REQUIRE_DOWNLOAD === '1';
    process.stderr.write(
      `centag: postinstall download skipped (${e.message}). ` +
      `Binary will be fetched on first run.\n`
    );
    if (hard) process.exit(1);
  }
}

main();
