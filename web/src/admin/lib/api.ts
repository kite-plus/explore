import { useCallback } from 'react'
import {
  QueryClient,
  useMutation,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query'
import { toast } from 'sonner'
import { AdminUnauthorizedError, accountRequest, adminRequest } from '@/lib/admin-api'
import { useAuth } from '@/admin/context/auth-provider'

export function createQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        // A 401 has already signed the admin out; other errors get two retries.
        retry: (failureCount, error) =>
          !(error instanceof AdminUnauthorizedError) && failureCount < 2,
        staleTime: 10 * 1000,
      },
    },
  })
}

/** The message for a failed request, or null after a 401 signed the admin out. */
export function errorMessage(cause: unknown, fallback = '操作失败，请重试。') {
  if (cause instanceof AdminUnauthorizedError) return null
  return cause instanceof Error && cause.message ? cause.message : fallback
}

export function toastError(cause: unknown, fallback?: string) {
  const message = errorMessage(cause, fallback)
  if (message) toast.error(message)
}

function useSignedInRequest(send: typeof adminRequest) {
  const { status, onUnauthorized } = useAuth()
  return useCallback(
    <T>(path: string, init: RequestInit = {}): Promise<T> => {
      if (status !== 'signed-in') return Promise.reject(new AdminUnauthorizedError())
      return send<T>(path, onUnauthorized, init)
    },
    [send, status, onUnauthorized]
  )
}

/** Admin requests with the signed-in account's session. */
export function useAdminRequest() {
  return useSignedInRequest(adminRequest)
}

/** Requests about the signed-in account itself, under /api/v1/me. */
export function useAccountRequest() {
  return useSignedInRequest(accountRequest)
}

export function useAdminQuery<T>(
  path: string,
  options: { refetchInterval?: number; keepPrevious?: boolean } = {}
) {
  const request = useAdminRequest()
  return useQuery({
    queryKey: ['admin', path],
    queryFn: () => request<T>(path),
    refetchInterval: options.refetchInterval,
    placeholderData: options.keepPrevious ? (previous) => previous : undefined,
  })
}

/**
 * A write to the admin API that refreshes every admin query afterwards, so
 * lists, counts and the sidebar badges stay in step.
 */
export function useAdminMutation<TInput, TResult = void>(
  build: (input: TInput) => { path: string; method: string; body?: unknown }
) {
  const request = useAdminRequest()
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: TInput) => {
      const { path, method, body } = build(input)
      return request<TResult>(path, {
        method,
        body: body === undefined ? undefined : JSON.stringify(body),
      })
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: ['admin'] }),
  })
}
