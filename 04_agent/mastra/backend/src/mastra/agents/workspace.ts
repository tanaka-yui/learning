import { Workspace, LocalFilesystem } from "@mastra/core/workspace";
import { fileURLToPath } from "node:url";
import path from "node:path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

export const workspace = new Workspace({
  filesystem: new LocalFilesystem({ basePath: process.env.WORKSPACE_BASE_PATH ?? __dirname }),
  skills: ["./skills"],
  bm25: true,
});
