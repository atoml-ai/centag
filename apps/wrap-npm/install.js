// install.js
// npm postinstall: pre-download the platform binary so `npx centag-wrap`
// works offline after install. Failures are non-fatal (lazy download at runtime)
// unless CENTAG_WRAP_REQUIRE_DOWNLOAD=1 is set.

import { ensureBinary } from './lib/download.js';

async function main() {
  if (process.env.CENTAG_WRAP_SKIP_DOWNLOAD === '1') {
    process.stderr.write('centag-wrap: skip download (CENTAG_WRAP_SKIP_DOWNLOAD=1)\n');
    return;
  }
  try {
    const p = await ensureBinary();
    process.stderr.write(`centag-wrap: installed at ${p}\n`);
  } catch (e) {
    const hard = process.env.CENTAG_WRAP_REQUIRE_DOWNLOAD === '1';
    process.stderr.write(
      `centag-wrap: postinstall download skipped (${e.message}). ` +
      `Binary will be fetched on first run.\n`
    );
    if (hard) process.exit(1);
  }
}

main();
