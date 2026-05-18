#!/usr/bin/env node
"use strict";

const { spawnSync } = require("child_process");
const path = require("path");

const binaryPath = path.join(__dirname, "opencodereview");

const result = spawnSync(binaryPath, process.argv.slice(2), {
  stdio: "inherit",
  env: process.env,
});

process.exit(result.status ?? (result.error ? 1 : 0));
