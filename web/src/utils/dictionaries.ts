export const trainingModeOptions = ['全日制', '非全日制', '定向', '其他'];

export const industryOptions = [
  '党政机关',
  '事业单位',
  '国有企业',
  '民营企业',
  '高校科研',
  '金融',
  '医疗卫生',
  '社会组织',
  '其他',
];

export const genderOptions = ['男', '女', '未填'];

export const trainingLevelOptions = [
  { value: 'undergraduate', label: '本科' },
  { value: 'master', label: '硕士' },
  { value: 'doctor', label: '博士' },
] as const;

export const storedTrainingLevelOptions = [
  ...trainingLevelOptions,
  { value: 'unverified', label: '待核验' },
] as const;
