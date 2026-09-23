import type { APIRoute } from "astro";

import { apiURL } from "@/lib/config";

/**
 * 通配符代理：将所有 /api/v1/admin/* 请求转发至后端 Gin 服务
 * - Authorization 头原样透传（前端负责携带 Bearer Token）
 * - query string 原样透传（用于 ?status= ?health= ?exclude= 等参数）
 * - 支持所有 HTTP 方法：GET / POST / PATCH / DELETE
 */
export const ALL: APIRoute = async ({ request, params }) => {
  const path = params.path ?? "";
  const url = new URL(request.url);
  const target = `${apiURL()}/api/v1/admin/${path}${url.search}`;
  const headers = new Headers();
  for (const name of ["authorization", "content-type", "accept-language", "cookie", "x-csrf-token"]) {
    const value = request.headers.get(name);
    if (value) headers.set(name, value);
  }

  let response: Response;
  try {
    response = await fetch(target, {
      method: request.method,
      headers,
      body: ["GET", "HEAD"].includes(request.method) ? undefined : request.body,
      // Node fetch 的 duplex 选项，支持流式请求体
      // @ts-expect-error Node 特有选项
      duplex: "half",
      signal: AbortSignal.timeout(35_000),
    });
  } catch {
    return new Response(JSON.stringify({ error: { code: "unavailable" } }), {
      status: 502,
      headers: { "Content-Type": "application/json" },
    });
  }

  // 透传 Content-Type，其他响应头不透传（避免双重 CORS 等问题）
  const responseHeaders = new Headers({
    "Content-Type": response.headers.get("Content-Type") ?? "application/json",
    "Cache-Control": "no-store",
  });

  return new Response(response.body, {
    status: response.status,
    headers: responseHeaders,
  });
};
