import { listTasks } from "../tools/taskTools.js";

const priorityOrder: Record<string, number> = { high: 0, medium: 1, low: 2 };

export const prioritize = (): string => {
  const tasks = listTasks().filter((t) => t.status !== "done");
  const sorted = [...tasks].sort(
    (a, b) => priorityOrder[a.priority] - priorityOrder[b.priority]
  );
  if (sorted.length === 0) return "未完了のタスクはありません。";
  return sorted
    .map((t, i) => `${i + 1}. [${t.priority}] ${t.title} (${t.status})`)
    .join("\n");
};
