import { listTasks } from "../tools/taskTools.js";

export const summarize = (): string => {
  const tasks = listTasks();
  const done = tasks.filter((t) => t.status === "done").length;
  const inProgress = tasks.filter((t) => t.status === "in_progress").length;
  const todo = tasks.filter((t) => t.status === "todo").length;
  return `タスク合計: ${tasks.length}件（完了: ${done}件、進行中: ${inProgress}件、未着手: ${todo}件）`;
};
