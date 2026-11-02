import { describe, expect, it } from 'vitest';
import lockfileText from '../../package-lock.json?raw';
import packageJsonText from '../../package.json?raw';

// follow-redirects 是 axios 的传递依赖，Web 端全部 HTTP 请求都经由它处理重定向，
// 历史上多次命中重定向凭据泄露类高危公告（GHSA-q3jh-wrc6-jgq8、GHSA-8r9p-f939-6h3c、
// GHSA-mrwp-f3q8-3g9p）。
//
// 这里不把版本写死在测试里：`web/package.json` 的 `overrides` 是唯一事实来源，
// 测试只校验「锁文件与声明一致 + 生产树只有一份 + 仍满足 axios 声明范围」。
// 公告再次出现时改 overrides 并重新生成锁文件即可，测试无需改动；
// 版本是否安全由 CI 的 `npm audit --omit=dev --audit-level=high` 判定。
const packageName = 'follow-redirects';

interface LockfileEntry {
  version?: string;
  resolved?: string;
  dependencies?: Record<string, string>;
}

interface Lockfile {
  packages: Record<string, LockfileEntry>;
}

interface PackageJson {
  overrides?: Record<string, string>;
}

type Version = [number, number, number];

const lockfile = JSON.parse(lockfileText) as Lockfile;
const packageJson = JSON.parse(packageJsonText) as PackageJson;

const pinnedVersion = packageJson.overrides?.[packageName];
const lockEntry = lockfile.packages[`node_modules/${packageName}`];
const copies = Object.entries(lockfile.packages)
  .filter(
    ([path]) =>
      path === `node_modules/${packageName}` || path.endsWith(`/node_modules/${packageName}`),
  )
  .map(([path, entry]) => ({ path, version: entry.version }));

function parseVersion(value: string): Version {
  const match = /^(\d+)\.(\d+)\.(\d+)(?:[-+].*)?$/.exec(value.trim());
  if (!match) {
    throw new Error(`无法解析版本号：${value}`);
  }
  return [Number(match[1]), Number(match[2]), Number(match[3])];
}

function compareVersions(left: Version, right: Version): number {
  for (let index = 0; index < left.length; index += 1) {
    const difference = left[index] - right[index];
    if (difference !== 0) return difference;
  }
  return 0;
}

// ^1.16.0 的上界是 2.0.0；^0.4.2 的上界是 0.5.0；^0.0.3 的上界是 0.0.4。
function caretUpperBound([major, minor, patch]: Version): Version {
  if (major > 0) return [major + 1, 0, 0];
  if (minor > 0) return [0, minor + 1, 0];
  return [0, 0, patch + 1];
}

function tildeUpperBound([major, minor]: Version): Version {
  return [major, minor + 1, 0];
}

// 只支持单个比较符的简单范围；出现复合范围时直接报错，避免把「不支持」当成「满足」。
function satisfiesRange(version: string, range: string): boolean {
  const trimmed = range.trim();
  const match = /^(\^|~|>=|<=|>|<|=)?(\d+\.\d+\.\d+)$/.exec(trimmed);
  if (!match) {
    throw new Error(`不支持解析范围：${range}`);
  }
  const operator = match[1] ?? '=';
  const current = parseVersion(version);
  const bound = parseVersion(match[2]);
  const order = compareVersions(current, bound);
  switch (operator) {
    case '^':
      return order >= 0 && compareVersions(current, caretUpperBound(bound)) < 0;
    case '~':
      return order >= 0 && compareVersions(current, tildeUpperBound(bound)) < 0;
    case '>=':
      return order >= 0;
    case '>':
      return order > 0;
    case '<=':
      return order <= 0;
    case '<':
      return order < 0;
    default:
      return order === 0;
  }
}

describe('web dependency security baseline', () => {
  it('pins follow-redirects to an exact version through package.json overrides', () => {
    expect(pinnedVersion, `${packageName} 必须在 package.json 的 overrides 中固定`).toBeDefined();
    // overrides 用范围声明时，「锁文件与声明一致」会退化成范围比较，固定不住实际版本。
    expect(pinnedVersion).toMatch(/^\d+\.\d+\.\d+$/);
  });

  it('resolves the pinned follow-redirects version in the lockfile', () => {
    expect(lockEntry, `${packageName} 必须留在锁文件里`).toBeDefined();
    expect(lockEntry?.version).toBe(pinnedVersion);
    // 锁文件被手工编辑却不更新 resolved 时，这里会先失败。
    expect(lockEntry?.resolved).toContain(`${packageName}-${lockEntry?.version}.tgz`);
  });

  it('keeps a single follow-redirects copy in the production tree', () => {
    // 出现嵌套副本意味着某个依赖绕过了 overrides，实际运行的版本可能不是被固定的那个。
    expect(copies).toHaveLength(1);
    expect(copies[0]?.path).toBe(`node_modules/${packageName}`);
  });

  it('keeps the pinned version inside the range axios declares', () => {
    const axiosRange = lockfile.packages['node_modules/axios']?.dependencies?.[packageName];
    expect(axiosRange, 'axios 必须仍在依赖 follow-redirects').toBeDefined();
    // axios 收紧范围时这里会先失败，而不是等 overrides 与解除解析冲突。
    expect(satisfiesRange(lockEntry?.version ?? '0.0.0', axiosRange ?? '')).toBe(true);
  });
});
