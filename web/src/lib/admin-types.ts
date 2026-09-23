// 与后端 admin.go 的数据结构严格对应
// 更新时需同步检查 internal/api/admin.go

import type { CheckReport } from "@/lib/types";

export type { CheckReport };

/** 对应后端 adminSubmissionJSON（嵌入 submissionJSON） */
export interface AdminSubmission {
  id: string;
  status: "pending" | "approved" | "rejected";
  host: string;
  site_url: string;
  feed_url: string;
  check_report: CheckReport | null;
  /** 审核驳回原因（approved/rejected 时有值） */
  review_note: string;
  /** 提交者备注说明 */
  note: string;
  /** 审核者标识（如 alice）*/
  reviewed_by: string;
  created_at: string; // ISO 8601
  reviewed_at: string | null;
}

/** 对应后端 adminBlogJSON */
export interface AdminBlog {
  host: string;
  name: string;
  description: string;
  site_url: string;
  feed_url: string;
  language: string;
  generator: string;
  status: "active" | "paused";
  status_note: string;
  show_excerpt: boolean;
  extra_domains: string[];
  default_tags: string[];
  /** 前端计算：active + gone_since==null + 30 天内成功抓取 */
  visible: boolean;
  fetch_interval_seconds: number;
  next_fetch_at: string;
  last_fetched_at: string | null;
  last_succeeded_at: string | null;
  consecutive_failures: number;
  last_error: string;
  gone_since: string | null;
  created_at: string;
}

export interface FetchAttempt {
  id: number;
  started_at: string;
  finished_at: string | null;
  outcome: "running" | "changed" | "unchanged" | "failed";
  http_status: number | null;
  entry_count: number | null;
  error: string;
}

export interface FetchQueueItem {
  host: string;
  name: string;
  queue_status: "queued" | "running" | "retry" | "scheduled" | "paused" | "stalled";
  next_fetch_at: string;
  last_fetched_at: string | null;
  last_succeeded_at: string | null;
  consecutive_failures: number;
  last_error: string;
  last_attempt: FetchAttempt | null;
}

export interface FetchQueueSnapshot {
  worker_online: boolean;
  worker_count: number;
  worker_last_seen_at: string | null;
  data: FetchQueueItem[];
}

/** 对应后端 excludedJSON */
export interface ExcludedHost {
  host: string;
  reason: "opt_out" | "blocked";
  note: string;
  created_at: string;
}

/** 审批通过请求体（对应 approveRequest） */
export interface ApprovePayload {
  name?: string;
  language?: string;
  feed_url?: string;
  extra_domains?: string[];
  show_excerpt?: boolean;
}

/** 博客元数据更新请求体（对应 updateBlogRequest） */
export interface UpdateBlogPayload {
  name?: string;
  language?: string;
  feed_url?: string;
  extra_domains?: string[];
  default_tags?: string[];
  show_excerpt?: boolean;
  status?: "active" | "paused";
  status_note?: string;
}

/** 直接创建博客请求体（对应 createBlogRequest） */
export interface CreateBlogPayload {
  site_url: string;
  feed_url?: string;
  name?: string;
  language?: string;
  extra_domains?: string[];
  show_excerpt?: boolean;
}
