import { useQuery, useMutation } from '@tanstack/react-query'
import { messagingApi, type ChatTask } from '../api/messaging'
import { queryClient } from '../lib/query-client'
import { useAuthStore } from '../stores/auth.store'
import { keys } from './keys'

// --- Tasks ---

export function useTasks(channelId: string, status?: string) {
  return useQuery({
    queryKey: keys.messaging.tasks(channelId, status),
    queryFn: () => messagingApi.listTasks(channelId, status),
    enabled: !!channelId,
  })
}

/** Creates a task with optimistic insert into task list. */
export function useCreateTask(channelId: string) {
  return useMutation({
    meta: { silentError: true },
    mutationFn: (data: { title: string; assignee_id?: string; due_date?: string }) =>
      messagingApi.createTask(channelId, data),
    onMutate: async (data) => {
      await queryClient.cancelQueries({ queryKey: keys.messaging.tasksOf(channelId) })
      // The list is cached per status filter, so snapshot and patch every one.
      const previous = queryClient.getQueriesData<{ tasks: ChatTask[] }>({ queryKey: keys.messaging.tasksOf(channelId) })
      const user = useAuthStore.getState().user

      const tempTask: ChatTask = {
        id: `temp-${Date.now()}`,
        message_id: '',
        channel_id: channelId,
        title: data.title,
        assignee_id: data.assignee_id || '',
        assignee_name: '',
        status: 'open',
        due_date: data.due_date,
        created_by: user?.id || '',
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
      }

      queryClient.setQueriesData<{ tasks: ChatTask[] }>(
        { queryKey: keys.messaging.tasksOf(channelId) },
        (old) => old ? { ...old, tasks: [tempTask, ...old.tasks] } : old,
      )
      return { previous }
    },
    onSuccess: (serverTask) => {
      queryClient.setQueriesData<{ tasks: ChatTask[] }>(
        { queryKey: keys.messaging.tasksOf(channelId) },
        (old) => {
          if (!old) return old
          return {
            ...old,
            tasks: old.tasks.map((t) => t.id.startsWith('temp-') ? serverTask : t),
          }
        },
      )
    },
    onError: (_err, _vars, context) => {
      context?.previous.forEach(([key, data]) => queryClient.setQueryData(key, data))
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: keys.messaging.tasksOf(channelId) }),
  })
}

/** Updates a task with optimistic field change. */
export function useUpdateTask(channelId: string) {
  return useMutation({
    meta: { silentError: true },
    mutationFn: ({ taskId, ...data }: { taskId: string; status?: string; assignee_id?: string; title?: string; due_date?: string }) =>
      messagingApi.updateTask(taskId, data),
    onMutate: async ({ taskId, ...data }) => {
      await queryClient.cancelQueries({ queryKey: keys.messaging.tasksOf(channelId) })
      // The list is cached per status filter, so snapshot and patch every one.
      const previous = queryClient.getQueriesData<{ tasks: ChatTask[] }>({ queryKey: keys.messaging.tasksOf(channelId) })

      queryClient.setQueriesData<{ tasks: ChatTask[] }>(
        { queryKey: keys.messaging.tasksOf(channelId) },
        (old) => {
          if (!old) return old
          return {
            ...old,
            tasks: old.tasks.map((t) =>
              t.id === taskId ? { ...t, ...data, updated_at: new Date().toISOString() } : t,
            ),
          }
        },
      )
      return { previous }
    },
    onError: (_err, _vars, context) => {
      context?.previous.forEach(([key, data]) => queryClient.setQueryData(key, data))
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: keys.messaging.tasksOf(channelId) }),
  })
}
