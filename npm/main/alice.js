#!/usr/bin/env node
'use strict';

const { execFileSync } = require('child_process');
const os = require('os');
const path = require('path');

const PLATFORM_MAP = {
  'linux-x64':    { pkg: '@alice_space/alice-linux-x64',   bin: 'alice' },
  'linux-arm64':  { pkg: '@alice_space/alice-linux-arm64',  bin: 'alice' },
  'darwin-x64':   { pkg: '@alice_space/alice-darwin-x64',   bin: 'alice' },
  'darwin-arm64': { pkg: '@alice_space/alice-darwin-arm64', bin: 'alice' },
  'win32-x64':    { pkg: '@alice_space/alice-win32-x64',    bin: 'alice.exe' },
};

const platformKey = `${os.platform()}-${os.arch()}`;
const entry = PLATFORM_MAP[platformKey];

if (!entry) {
  console.error(`[alice] Unsupported platform: ${platformKey}`);
  console.error(`Supported platforms: ${Object.keys(PLATFORM_MAP).join(', ')}`);
  process.exit(1);
}

let binPath;
try {
  binPath = require.resolve(`${entry.pkg}/bin/${entry.bin}`);
} catch {
  console.error(`[alice] Binary package not found: ${entry.pkg}`);
  console.error('Try reinstalling: npm install -g @alice_space/alice');
  process.exit(1);
}

try {
  execFileSync(binPath, process.argv.slice(2), { stdio: 'inherit' });
} catch (e) {
  process.exit(e.status ?? 1);
}
