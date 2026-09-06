import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";

import { describe, expect, it } from "vitest";

const currentDir = dirname(fileURLToPath(import.meta.url));
// GroupsView.vue 已拆分：模型列表弹层的模板/样式现分别位于
// GroupCreateModal.vue 与 GroupEditModal.vue 中，因此这里合并三者的源码
// 一并断言，保持原有断言内容不变（零功能损失）。
const groupsViewSource = [
  resolve(currentDir, "../GroupsView.vue"),
  resolve(currentDir, "../../../components/admin/group/GroupCreateModal.vue"),
  resolve(currentDir, "../../../components/admin/group/GroupEditModal.vue"),
]
  .map((filePath) => readFileSync(filePath, "utf8"))
  .join("\n");

describe("groups models list layout", () => {
  it("keeps the toolbar outside of the scrolling list content", () => {
    expect(groupsViewSource).toContain("overflow-hidden rounded-lg border");
    expect(groupsViewSource).toContain("max-h-64 space-y-2 overflow-y-auto p-2");
    expect(groupsViewSource).not.toContain("sticky top-0");
  });

  it("uses a wide dialog and keeps model pricing controls responsive", () => {
    expect(groupsViewSource).toContain('width="wide"');
    expect(groupsViewSource).toContain(
      "btn btn-secondary shrink-0 whitespace-nowrap",
    );
  });
});
