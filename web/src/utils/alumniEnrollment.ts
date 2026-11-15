import type { DataDomain } from '../types/auth';
import type { TrainingLevel } from '../types/alumni';
import { TRAINING_LEVEL_DOMAIN_CODES } from '../types/alumni';

/** 判断培养层次与数据域编码是否组成合法组合，规则与服务端保持一致。 */
export function isTrainingLevelDomainCode(
  level: TrainingLevel | '' | undefined,
  code: string | undefined,
): boolean {
  if (!level || !code) return false;
  return TRAINING_LEVEL_DOMAIN_CODES[level].includes(code);
}

/** 返回当前权限内与所选培养层次兼容的培养类别。 */
export function domainsForTrainingLevel(
  domains: DataDomain[],
  level: TrainingLevel | '' | undefined,
): DataDomain[] {
  if (!level) return domains;
  return domains.filter((domain) => isTrainingLevelDomainCode(level, domain.code));
}

/**
 * 层次变化后的类别选择：
 * 原类别仍合法时保留；否则仅在唯一合法且有权限时自动选中，多个合法类别交由管理员选择。
 */
export function resolveDomainForLevel(
  domains: DataDomain[],
  level: TrainingLevel | '',
  currentDomainID: number | undefined,
): { domainID?: number; needsChoice: boolean } {
  const current = domains.find((domain) => domain.id === currentDomainID);
  if (current && isTrainingLevelDomainCode(level, current.code)) {
    return { domainID: current.id, needsChoice: false };
  }
  const allowed = domainsForTrainingLevel(domains, level);
  if (allowed.length === 1) {
    return { domainID: allowed[0].id, needsChoice: false };
  }
  return { domainID: undefined, needsChoice: allowed.length > 1 };
}
