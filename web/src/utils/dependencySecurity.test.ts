import { describe, expect, it } from 'vitest';
import lockfileText from '../../package-lock.json?raw';

// follow-redirects 是 axios 的传递依赖，所有 Web 端 HTTP 请求都经由它处理重定向。
// 1.16.0 命中两条高危公告：跨域重定向时敏感请求头可能被错误保留并转发
// （GHSA-q3jh-wrc6-jgq8、GHSA-8r9p-f939-6h3c），1.16.1 修复。
//
// 这里把版本下限钉死，避免后续重新生成锁文件或升级 axios 时静默退回含漏洞的版本，
// 让 npm audit 的结论不再只是「碰巧是绿的」。
const minimumFollowRedirectsVersion = '1.16.1';

interface LockfileEntry {
  version?: string;
  dependencies?: Record<string, string>;
}

interface Lockfile {
  packages: Record<string, LockfileEntry>;
}

const lockfile = JSON.parse(lockfileText) as Lockfile;

function compareVersions(left: string, right: string): number {
  const leftParts = left.split('.').map((part) => Number.parseInt(part, 10));
  const rightParts = right.split('.').map((part) => Number.parseInt(part, 10));
  for (let index = 0; index < Math.max(leftParts.length, rightParts.length); index += 1) {
    const difference = (leftParts[index] ?? 0) - (rightParts[index] ?? 0);
    if (difference !== 0) return difference;
  }
  return 0;
}

describe('web dependency security baseline', () => {
  it('locks follow-redirects at or above the version that fixes the redirect advisories', () => {
    const entry = lockfile.packages['node_modules/follow-redirects'];
    expect(entry, 'follow-redirects must stay in the lockfile').toBeDefined();
    expect(entry?.version).toBeDefined();
    expect(
      compareVersions(entry?.version ?? '0.0.0', minimumFollowRedirectsVersion),
      `follow-redirects ${entry?.version} is older than ${minimumFollowRedirectsVersion}`,
    ).toBeGreaterThanOrEqual(0);
  });

  it('keeps a single follow-redirects copy in the production tree', () => {
    const copies = Object.entries(lockfile.packages)
      .filter(([path]) => path.endsWith('node_modules/follow-redirects'))
      .map(([path, entry]) => `${path}@${entry.version ?? 'unknown'}`);
    expect(copies).toEqual(['node_modules/follow-redirects@1.16.1']);
  });

  it('lets axios resolve to the fixed follow-redirects version', () => {
    const axiosRange = lockfile.packages['node_modules/axios']?.dependencies?.['follow-redirects'];
    expect(axiosRange).toBeDefined();
    // axios 声明 ^1.16.0，覆盖 1.16.1；若将来收紧范围，这里会先失败。
    expect(axiosRange).toMatch(/^\^?1\.16\./);
  });
});
