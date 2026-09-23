// Mirrors docs/design/api.md. These move to generated types once
// api/openapi.yaml exists.

export interface BlogRef {
  host: string;
  name: string;
  site_url: string;
  language: string;
}

export interface Entry {
  id: string;
  title: string;
  url: string;
  excerpt: string | null;
  published_at: string | null;
  blog?: BlogRef;
}

export interface Blog {
  host: string;
  name: string;
  site_url: string;
  feed_url: string;
  language: string;
  generator: string;
  last_published_at: string | null;
}

export interface Page<T> {
  data: T[];
  next_cursor: string | null;
}

export interface BlogPage {
  blog: Blog;
  entries: Entry[];
}

export type Severity = "error" | "warning" | "info";

export interface Problem {
  code: string;
  severity: Severity;
  count?: number;
  detail?: string;
  hint?: string;
}

export interface CheckReport {
  input_url: string;
  feed_url?: string;
  discovered_by?: string;
  format?: string;
  generator?: string;
  title?: string;
  language?: string;
  problems: Problem[];
  passed: boolean;
}

export interface Submission {
  id: string;
  status: "pending" | "approved" | "rejected";
  host: string;
  site_url: string;
  feed_url: string;
  check_report: CheckReport | null;
  review_note?: string;
  created_at: string;
  reviewed_at: string | null;
}

export interface ApiError {
  error: { code: string; message: string };
  submission_id?: string;
  check_report?: CheckReport;
}
