import { v4 as uuidv4 } from "uuid";

type Priority = "low" | "medium" | "high";
type Status = "todo" | "in_progress" | "done";

export type Task = {
  id: string;
  title: string;
  description: string;
  priority: Priority;
  status: Status;
  createdAt: string;
};

export const taskStore: Task[] = [];

type CreateInput = { title: string; description: string; priority: Priority };

export const createTask = (input: CreateInput): Task => {
  const task: Task = {
    id: uuidv4(),
    ...input,
    status: "todo",
    createdAt: new Date().toISOString(),
  };
  taskStore.push(task);
  return task;
};

export const listTasks = (): Task[] => [...taskStore];

export const updateTask = (
  id: string,
  updates: Partial<Omit<Task, "id" | "createdAt">>
): Task | null => {
  const idx = taskStore.findIndex((t) => t.id === id);
  if (idx === -1) return null;
  taskStore[idx] = { ...taskStore[idx], ...updates };
  return taskStore[idx];
};

export const deleteTask = (id: string): boolean => {
  const idx = taskStore.findIndex((t) => t.id === id);
  if (idx === -1) return false;
  taskStore.splice(idx, 1);
  return true;
};
