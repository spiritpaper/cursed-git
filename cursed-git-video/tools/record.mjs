#!/usr/bin/env node
/**
 * record.mjs: turn an animation page into a crisp MP4 for YouTube.
 *
 * It opens the page in headless Chromium with ?video, freezes time, then steps the page forward
 * exactly one frame at a time and screenshots each frame. Because time is simulated, the video is
 * perfectly smooth even if your computer is slow, and identical every run.
 *
 * Usage:
 *   node tools/record.mjs <file-or-url> [options]
 *
 * Options:
 *   --seconds 30        length of the video (default 30)
 *   --vertical          1080x1920 for Shorts/Reels (default is 1920x1080)
 *   --theme dark|light  force a color theme (default: dark)
 *   --fps 30            frames per second (default 30; 60 for extra smooth)
 *   --query "a=1&b=2"   extra URL parameters, e.g. "scenario=crash" or "scene=galton"
 *   --out file.mp4      output path (default: <page name>[-vertical].mp4 next to the page)
 *   --scale 1           device scale factor; 2 renders at double resolution (4K from 1080p layout)
 *   --realtime          record in real time instead (faster to make, a bit less sharp)
 *
 * Needs: Node 18+, `npm i playwright`, `npx playwright install chromium`, and ffmpeg on PATH.
 */
import { chromium } from 'playwright';
import { spawn } from 'node:child_process';
import { existsSync, mkdtempSync, readdirSync, rmSync } from 'node:fs';
import { resolve, basename, dirname, join, extname } from 'node:path';
import { pathToFileURL } from 'node:url';
import { tmpdir } from 'node:os';

// ---------- arguments ----------
const argv = process.argv.slice(2);
if (!argv.length || argv.includes('--help')) {
  console.log('Usage: node tools/record.mjs <file-or-url> [--seconds 30] [--vertical] [--theme dark|light] [--fps 30] [--query "k=v"] [--out out.mp4] [--scale 1] [--realtime]');
  process.exit(0);
}
const opt = (name, def) => { const i = argv.indexOf('--' + name); return i >= 0 ? argv[i + 1] : def; };
const flag = name => argv.includes('--' + name);
const target = argv[0];
const seconds = +opt('seconds', 30);
const fps = +opt('fps', 30);
const vertical = flag('vertical');
const theme = opt('theme', 'dark');
const scale = +opt('scale', 1);
const realtime = flag('realtime');
const extra = opt('query', '');
const [W, H] = vertical ? [1080, 1920] : [1920, 1080];

const isUrl = /^https?:\/\//.test(target);
if (!isUrl && !existsSync(target)) { console.error(`File not found: ${target}`); process.exit(1); }
const base = isUrl ? target : pathToFileURL(resolve(target)).href;
const params = new URLSearchParams(extra);
params.set('video', vertical ? 'vertical' : '1');
params.set('theme', theme);
const url = base + (base.includes('?') ? '&' : '?') + params.toString().replace('video=1', 'video');
const out = resolve(opt('out', isUrl ? `recording${vertical ? '-vertical' : ''}.mp4`
  : join(dirname(target), basename(target, extname(target)) + (vertical ? '-vertical' : '') + '.mp4')));

// ---------- helpers ----------
function ffmpeg(args) {
  const p = spawn('ffmpeg', ['-y', '-loglevel', 'error', ...args], { stdio: ['pipe', 'inherit', 'inherit'] });
  p.on('error', e => { console.error('Could not start ffmpeg. Is it installed and on your PATH?', e.message); process.exit(1); });
  return p;
}
const done = p => new Promise((res, rej) => p.on('close', c => c === 0 ? res() : rej(new Error('ffmpeg exited with code ' + c))));
function progress(i, total) {
  const pct = Math.round(i / total * 100);
  process.stdout.write(`\r  frame ${i}/${total}  [${'#'.repeat(pct / 4).padEnd(25)}] ${pct}%`);
}

// ---------- record ----------
// CHROMIUM_PATH lets you use a system Chromium where Playwright has no build (e.g. Ubuntu 26.04: CHROMIUM_PATH=/snap/bin/chromium)
const browser = await chromium.launch(process.env.CHROMIUM_PATH ? { executablePath: process.env.CHROMIUM_PATH } : {});
try {
  if (realtime) {
    const dir = mkdtempSync(join(tmpdir(), 'rec-'));
    const ctx = await browser.newContext({ viewport: { width: W, height: H }, colorScheme: theme,
      recordVideo: { dir, size: { width: W, height: H } } });
    const page = await ctx.newPage();
    await page.goto(url);
    console.log(`Recording ${seconds}s in real time…`);
    await page.waitForTimeout(seconds * 1000);
    await ctx.close();
    const webm = join(dir, readdirSync(dir).find(f => f.endsWith('.webm')));
    await done(ffmpeg(['-i', webm, '-c:v', 'libx264', '-pix_fmt', 'yuv420p', '-crf', '18', '-r', String(fps), out]));
    rmSync(dir, { recursive: true, force: true });
  } else {
    const ctx = await browser.newContext({ viewport: { width: W, height: H }, deviceScaleFactor: scale, colorScheme: theme });
    const page = await ctx.newPage();
    // Freeze JS time (Date, setTimeout, requestAnimationFrame, performance.now) so only we move it forward.
    const t0 = new Date('2026-01-01T00:00:00Z').getTime();
    await page.clock.install({ time: t0 });
    await page.clock.pauseAt(t0 + 10);
    await page.goto(url, { waitUntil: 'load' });
    await page.evaluate(() => document.fonts && document.fonts.ready);
    await page.waitForTimeout(150);

    const total = Math.round(seconds * fps), dt = 1000 / fps;
    console.log(`Rendering ${total} frames at ${W * scale}x${H * scale}, ${fps} fps → ${out}`);
    const enc = ffmpeg(['-f', 'image2pipe', '-framerate', String(fps), '-i', '-',
      '-c:v', 'libx264', '-preset', 'medium', '-crf', '16', '-pix_fmt', 'yuv420p', '-movflags', '+faststart', out]);
    const finished = done(enc);

    for (let i = 0; i < total; i++) {
      await page.clock.runFor(dt);
      // CSS transitions/animations run on the real clock, so pause them and step them by hand too.
      await page.evaluate(step => {
        for (const a of document.getAnimations()) {
          if (a.__t == null) { a.pause(); a.__t = 0; }   // take over this animation's clock
          a.__t += step;
          const end = a.effect && a.effect.getComputedTiming().endTime;
          if (isFinite(end) && a.__t >= end) { try { a.finish(); } catch (e) {} }  // lets finish handlers run
          else a.currentTime = a.__t;
        }
      }, dt);
      const png = await page.screenshot({ type: 'png' });
      if (!enc.stdin.write(png)) await new Promise(r => enc.stdin.once('drain', r));
      progress(i + 1, total);
    }
    enc.stdin.end();
    await finished;
    process.stdout.write('\n');
  }
  console.log(`Saved ${out}`);
} finally {
  await browser.close();
}
