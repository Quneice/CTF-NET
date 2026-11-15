#!/usr/bin/env node
/**
 * 下载便携 Chromium 并部署到 thirdparty/chromium/(内置浏览器离线部署)。
 *
 * 用法:node scripts/fetch-portable-chromium.mjs [目标目录]
 * 缺省目标:仓库根 thirdparty/chromium/(内置浏览器探测会在此找 chrome.exe)。
 *
 * 原理:临时目录 npm i playwright → npx playwright install chromium
 *       → 从 ms-playwright 的 chromium 版本目录下 chrome-win 子目录整目录拷贝到目标。
 * 产物 ~170MB,不提交 git(thirdparty/ 已在 .gitignore)。
 */
import { execSync } from 'node:child_process';
import { existsSync, mkdirSync, cpSync, rmSync, readdirSync, statSync } from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const dest = process.argv[2] ? path.resolve(process.argv[2]) : path.join(root, 'thirdparty', 'chromium');

function run(cmd, cwd, env = {}) {
  console.log(`> ${cmd}`);
  execSync(cmd, {
    cwd,
    stdio: 'inherit',
    shell: process.platform === 'win32' ? 'cmd.exe' : '/bin/sh',
    env: { ...process.env, ...env },
  });
}

if (existsSync(path.join(dest, 'chrome.exe'))) {
  console.log(`已存在,跳过:${dest}`);
  process.exit(0);
}

const tmp = path.join(os.tmpdir(), `ctfnet-chromium-${Date.now()}`);
mkdirSync(tmp, { recursive: true });
try {
  run('npm init -y', tmp);
  run('npm i playwright', tmp);
  // 官方 CDN(cdn.playwright.dev)在国内常被重置(ECONNRESET);默认先走 npmmirror 镜像,
  // 失败回退官方;亦可用 PLAYWRIGHT_DOWNLOAD_HOST 显式指定
  const mirrorHost = 'https://npmmirror.com/mirrors/playwright/';
  const explicit = process.env.PLAYWRIGHT_DOWNLOAD_HOST;
  try {
    run('npx playwright install chromium', tmp, {
      PLAYWRIGHT_DOWNLOAD_HOST: explicit || mirrorHost,
    });
  } catch (e) {
    if (explicit) throw e;
    console.log('镜像源失败,回退官方 CDN 重试…');
    run('npx playwright install chromium', tmp);
  }

  const msDir = path.join(process.env.LOCALAPPDATA || '', 'ms-playwright');
  if (!existsSync(msDir)) throw new Error(`未找到 ${msDir}`);
  // 新旧 playwright 布局不同:旧版 chrome-win/,新版(Chrome for Testing)chrome-win64/
  const found = readdirSync(msDir)
    .filter((d) => d.startsWith('chromium-'))
    .flatMap((d) => [path.join(msDir, d, 'chrome-win', 'chrome.exe'), path.join(msDir, d, 'chrome-win64', 'chrome.exe')])
    .find((p) => existsSync(p));
  if (!found) throw new Error('未找到 chrome.exe(chrome-win 与 chrome-win64 布局均不存在)');

  const srcDir = path.dirname(found);
  mkdirSync(dest, { recursive: true });
  cpSync(srcDir, dest, { recursive: true });
  const sizeMB = (statSync(found).size / 1048576).toFixed(1);
  console.log(`完成:${dest} (chrome.exe ${sizeMB}MB)`);
} finally {
  rmSync(tmp, { recursive: true, force: true });
}
