import { mkdir, writeFile } from 'node:fs/promises';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawn } from 'node:child_process';

const scriptDir = dirname(fileURLToPath(import.meta.url));
const webDir = resolve(scriptDir, '..');
const projectRoot = resolve(webDir, '..');
const projectName = process.env.E2E_COMPOSE_PROJECT || 'sdu-alumni-e2e';
const webPort = process.env.E2E_WEB_PORT || '18081';
const apiPort = process.env.E2E_API_PORT || '18080';
const resultDir = resolve(webDir, 'test-results');

const composeEnv = {
  ...process.env,
  MYSQL_ROOT_PASSWORD: process.env.E2E_MYSQL_ROOT_PASSWORD || 'e2e-root-password',
  MYSQL_PASSWORD: process.env.E2E_MYSQL_PASSWORD || 'e2e-db-password',
  MYSQL_HOST_PORT: process.env.E2E_MYSQL_PORT || '13307',
  REDIS_HOST_PORT: process.env.E2E_REDIS_PORT || '16379',
  API_HOST_PORT: apiPort,
  WEB_HOST_PORT: webPort,
  MINIO_API_HOST_PORT: process.env.E2E_MINIO_API_PORT || '19000',
  MINIO_CONSOLE_HOST_PORT: process.env.E2E_MINIO_CONSOLE_PORT || '19001',
  MINIO_ROOT_USER: process.env.E2E_MINIO_ACCESS_KEY || 'e2e-minio-admin',
  MINIO_ROOT_PASSWORD: process.env.E2E_MINIO_SECRET_KEY || 'e2e-minio-password',
};

function run(command, args, options = {}) {
  return new Promise((resolveRun, rejectRun) => {
    const child = spawn(command, args, {
      cwd: options.cwd || projectRoot,
      env: options.env || composeEnv,
      stdio: options.stdio || 'inherit',
    });
    child.on('error', rejectRun);
    child.on('close', (code) => {
      if (code === 0) {
        resolveRun();
        return;
      }
      rejectRun(new Error(`${command} ${args.join(' ')} exited with code ${code}`));
    });
  });
}

function composeArgs(...args) {
  return ['compose', '--project-name', projectName, ...args];
}

function wait(milliseconds) {
  return new Promise((resolveWait) => setTimeout(resolveWait, milliseconds));
}

async function waitForReady(url, label) {
  const deadline = Date.now() + 120_000;
  let lastError = 'no response';
  while (Date.now() < deadline) {
    try {
      const response = await fetch(url);
      if (response.ok) return;
      lastError = `status ${response.status}`;
    } catch (error) {
      lastError = error instanceof Error ? error.message : String(error);
    }
    await wait(500);
  }
  throw new Error(`${label} did not become ready: ${lastError}`);
}

async function saveComposeLogs() {
  await mkdir(resultDir, { recursive: true });
  const child = spawn('docker', composeArgs('logs', '--no-color'), {
    cwd: projectRoot,
    env: composeEnv,
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  let output = '';
  child.stdout.on('data', (chunk) => {
    output += chunk;
  });
  child.stderr.on('data', (chunk) => {
    output += chunk;
  });
  await new Promise((resolveLogs) => child.on('close', resolveLogs));
  await writeFile(resolve(resultDir, 'compose.log'), output);
}

let failed = false;
try {
  if (process.env.E2E_SKIP_BROWSER_INSTALL !== 'true') {
    await run('npx', ['playwright', 'install', 'chromium'], { cwd: webDir });
  }
  await run('docker', composeArgs('down', '--volumes', '--remove-orphans'));
  await run('docker', composeArgs('up', '--build', '--detach'));
  await waitForReady(`http://127.0.0.1:${apiPort}/api/v1/health/ready`, 'API');
  await waitForReady(`http://127.0.0.1:${webPort}/`, 'Web');
  await run('npm', ['run', 'test:e2e:run'], {
    cwd: webDir,
    env: { ...composeEnv, E2E_BASE_URL: `http://127.0.0.1:${webPort}` },
  });
} catch (error) {
  failed = true;
  await saveComposeLogs();
  throw error;
} finally {
  if (process.env.E2E_KEEP_ENV !== 'true') {
    await run('docker', composeArgs('down', '--volumes', '--remove-orphans')).catch(() => {});
  }
  if (failed) {
    console.error(`E2E diagnostics are available in ${resultDir}`);
  }
}
