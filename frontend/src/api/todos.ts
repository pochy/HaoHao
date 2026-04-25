import { readCookie } from './client'
import {
  createTodo,
  deleteTodo,
  listTodos,
  updateTodo,
} from './generated/sdk.gen'
import type {
  CreateTodoBodyWritable,
  TodoBody,
  UpdateTodoBodyWritable,
} from './generated/types.gen'

export async function fetchTodos(): Promise<TodoBody[]> {
  const data = await listTodos({
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as { items?: TodoBody[] | null }

  return data.items ?? []
}

export async function createTodoItem(body: CreateTodoBodyWritable): Promise<TodoBody> {
  return createTodo({
    headers: {
      'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '',
    },
    body,
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<TodoBody>
}

export async function updateTodoItem(
  todoPublicId: string,
  body: UpdateTodoBodyWritable,
): Promise<TodoBody> {
  return updateTodo({
    headers: {
      'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '',
    },
    path: { todoPublicId },
    body,
    responseStyle: 'data',
    throwOnError: true,
  }) as unknown as Promise<TodoBody>
}

export async function deleteTodoItem(todoPublicId: string): Promise<void> {
  await deleteTodo({
    headers: {
      'X-CSRF-Token': readCookie('XSRF-TOKEN') ?? '',
    },
    path: { todoPublicId },
    responseStyle: 'data',
    throwOnError: true,
  })
}
