import { CircleCheck, CircleX, Clock, ShieldCheck, ShieldX } from 'lucide-react'

export const submissionStatuses = [
  { label: '待审核', value: 'pending' as const, icon: Clock },
  { label: '已通过', value: 'approved' as const, icon: CircleCheck },
  { label: '已驳回', value: 'rejected' as const, icon: CircleX },
]

export const checkResults = [
  { label: '检查通过', value: 'passed' as const, icon: ShieldCheck },
  { label: '检查未通过', value: 'failed' as const, icon: ShieldX },
]

export const statusOrder = { pending: 0, approved: 1, rejected: 2 } as const

export const QUICK_REASONS = [
  '不是个人原创的独立博客',
  '没有可用的 RSS 或 Atom 订阅源',
  '超过一年没有更新',
  '内容以商业推广为主',
  '内容质量不符合收录标准',
  '暂不收录中英文以外的博客',
]
