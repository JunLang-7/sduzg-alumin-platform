import { describe, expect, it } from 'vitest';
import type { DataDomain } from '../types/auth';
import {
  domainsForTrainingLevel,
  isTrainingLevelDomainCode,
  resolveDomainForLevel,
} from './alumniEnrollment';

const domains: DataDomain[] = [
  { id: 1, code: 'undergraduate', name: '本科生' },
  { id: 2, code: 'academic_graduate', name: '学术学位研究生' },
  { id: 3, code: 'mpa', name: 'MPA专业学位研究生' },
];

describe('培养层次与培养类别组合', () => {
  it('本科只允许本科生类别', () => {
    expect(domainsForTrainingLevel(domains, 'undergraduate')).toEqual([domains[0]]);
    expect(isTrainingLevelDomainCode('undergraduate', 'mpa')).toBe(false);
  });

  it('硕士允许学术学位与 MPA，博士只允许学术学位', () => {
    expect(domainsForTrainingLevel(domains, 'master')).toEqual([domains[1], domains[2]]);
    expect(domainsForTrainingLevel(domains, 'doctor')).toEqual([domains[1]]);
  });

  it('待核验不约束类别，便于历史数据继续维护', () => {
    expect(domainsForTrainingLevel(domains, 'unverified')).toEqual(domains);
  });
});

describe('层次变化后的类别选择', () => {
  it('原类别仍合法时保留', () => {
    expect(resolveDomainForLevel(domains, 'doctor', 2)).toEqual({
      domainID: 2,
      needsChoice: false,
    });
  });

  it('唯一合法且有权限时自动选中', () => {
    expect(resolveDomainForLevel(domains, 'undergraduate', 3)).toEqual({
      domainID: 1,
      needsChoice: false,
    });
  });

  it('多个合法类别交由管理员选择', () => {
    expect(resolveDomainForLevel(domains, 'master', 1)).toEqual({
      domainID: undefined,
      needsChoice: true,
    });
  });

  it('没有合法类别时清空并要求重新选择', () => {
    expect(resolveDomainForLevel([domains[0], domains[2]], 'doctor', undefined)).toEqual({
      domainID: undefined,
      needsChoice: false,
    });
  });
});
